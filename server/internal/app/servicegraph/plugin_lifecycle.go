package servicegraph

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	plugingrants "github.com/RayleaBot/RayleaBot/server/internal/plugins/grants"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginmanifestrefresh "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifestrefresh"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

func buildPluginLifecycle(
	deps BuildDeps,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	renderer *renderservice.Service,
	grantView *plugingrants.View,
	runtimeRegistry *runtimeregistry.Registry,
	systemService *systemsvc.Service,
) *pluginservice.Controller {
	runtimeState := deps.Runtime
	return pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		RepoRoot:         runtimeState.RepoRoot(),
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		DesiredStateRepo: pluginStack.PluginRepository,
		Grants:           grantView,
		Runtimes:         runtimeRegistry,
		Dispatcher:       eventStack.Dispatcher,
		Scheduler:        platform.Scheduler,
		PluginConfig:     pluginStack.PluginConfig,
		Adapter:          eventStack.Adapter,
		Webhooks:         pluginStack.Webhooks,
		Tasks:            platform.Tasks,
		OnRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:  buildPluginLifecycleRefreshManifest(deps, pluginStack),
		SyncRenderTemplates: func(ctx context.Context) error {
			return renderplugintemplates.SyncCatalogRenderTemplates(ctx, renderer, pluginStack.Plugins)
		},
	})
}

func buildPluginLifecycleRefreshManifest(
	deps BuildDeps,
	pluginStack pluginstack.State,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginmanifestrefresh.RefreshPluginManifest(ctx, pluginStack.Plugins, pluginStack.PluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: deps.PluginValidator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Runtime.RuntimeLogger(),
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginStack.PluginRepository).(plugins.PackageMetadataLoader); ok {
				packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
				if err != nil {
					return nil, err
				}
				snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
			}
			return snapshots, nil
		})
	}
}

func buildPluginWebhookGateway(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	lifecycle *pluginservice.Controller,
	grantView *plugingrants.View,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Logger:        runtimeState.RuntimeLogger(),
		Registry:      pluginStack.Webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    eventStack.Dispatcher,
		Runtime:       lifecycle,
		Grants:        grantView,
	})
}
