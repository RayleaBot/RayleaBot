package apphost

import (
	"context"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func buildGovernanceService(runtimeState *appRuntimeState, pluginStack appPlugins, events *managementevents.GovernanceService) *governance.Service {
	return governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return runtimeState.Config },
		Plugins:        pluginStack.Plugins,
		BlacklistRepo:  pluginStack.blacklistRepo,
		WhitelistRepo:  pluginStack.whitelistRepo,
		WhitelistState: pluginStack.whitelistState,
		NotifyChanged:  events.PublishChanged,
	})
}

func buildLocalActionService(
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	grantView *pluginservice.GrantView,
	governanceService *governance.Service,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    func() config.Config { return runtimeState.Config },
		Logger:           runtimeState.Logger,
		RedactText:       runtimeState.redactString,
		Grants:           grantView,
		PluginConfig:     pluginStack.pluginConfig,
		PluginFiles:      pluginStack.pluginFiles,
		PluginKV:         pluginStack.pluginKV,
		Secrets:          platform.Secrets,
		Scheduler:        platform.Scheduler,
		Dispatcher:       pluginStack.Dispatcher,
		Renderer:         pluginStack.renderer,
		Adapter:          pluginStack.Adapter,
		PluginLogLimiter: pluginStack.pluginLogLimiter,
		Governance:       governanceService,
		ThirdParty:       thirdPartyService,
		BilibiliSession:  bilibiliSession,
	})
}

func configureLocalActionService(localActions *localaction.Service, pluginStack appPlugins) {
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		pluginservice.RefreshPluginCommands(pluginStack.Plugins, pluginStack.Dispatcher, pluginID, settings)
	})
}

func buildPluginLifecycleRefreshManifest(
	buildState appBuildState,
	runtimeState *appRuntimeState,
	pluginStack appPlugins,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginservice.RefreshPluginManifest(ctx, pluginStack.Plugins, pluginStack.pluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: buildState.pluginValidator,
				Roots:     buildState.discoverySpec.Roots,
				RepoRoot:  buildState.discoverySpec.RepoRoot,
				Logger:    runtimeState.Logger,
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginStack.pluginRepository).(plugins.PackageMetadataLoader); ok {
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
