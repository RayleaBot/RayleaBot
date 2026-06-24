package lifecycle

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type PlatformDeps struct {
	CurrentConfig    func() config.Config
	RepoRoot         string
	Logger           *slog.Logger
	Plugins          *plugincatalog.Catalog
	DesiredStateRepo plugins.DesiredStateRepository
	Runtimes         RuntimeRegistry
	Dispatcher       *dispatch.Dispatcher
	Scheduler        *scheduler.Engine
	PluginConfig     pluginconfig.Repository
	Adapter          BotIdentitySource
	Webhooks         *pluginwebhook.Registry
	Tasks            *tasks.Registry
	OnRecoveryChange func(string)
	Discovery        runtimepaths.PluginDiscoverySpec
	PluginValidator  *schema.Validator
	Renderer         *renderservice.Service
}

func NewPlatformController(deps PlatformDeps) *Controller {
	return NewController(Deps{
		CurrentConfig:       deps.CurrentConfig,
		RepoRoot:            deps.RepoRoot,
		Logger:              deps.Logger,
		Plugins:             deps.Plugins,
		DesiredStateRepo:    deps.DesiredStateRepo,
		Runtimes:            deps.Runtimes,
		Dispatcher:          deps.Dispatcher,
		Scheduler:           deps.Scheduler,
		PluginConfig:        deps.PluginConfig,
		Adapter:             deps.Adapter,
		Webhooks:            deps.Webhooks,
		Tasks:               deps.Tasks,
		OnRecoveryChange:    deps.OnRecoveryChange,
		RefreshManifest:     platformRefreshManifest(deps),
		SyncRenderTemplates: platformSyncRenderTemplates(deps),
	})
}

func platformRefreshManifest(deps PlatformDeps) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return refreshPluginManifest(ctx, deps.Plugins, deps.PluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: deps.PluginValidator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Logger,
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(deps.DesiredStateRepo).(plugins.PackageMetadataLoader); ok {
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

func platformSyncRenderTemplates(deps PlatformDeps) func(context.Context) error {
	return func(ctx context.Context) error {
		if deps.Renderer == nil || deps.Plugins == nil {
			return nil
		}
		return deps.Renderer.SyncPluginTemplateDeclarations(ctx, lifecycleRenderTemplateDeclarations(deps.Plugins.List()))
	}
}

func lifecycleRenderTemplateDeclarations(snapshots []plugins.Snapshot) []renderservice.PluginTemplateDeclaration {
	var declarations []renderservice.PluginTemplateDeclaration
	for _, snapshot := range snapshots {
		for _, declared := range snapshot.RenderTemplates {
			declarations = append(declarations, renderservice.PluginTemplateDeclaration{
				PluginID:          snapshot.PluginID,
				Path:              declared.Path,
				PackageRootPath:   snapshot.PackageRootPath,
				Valid:             snapshot.Valid,
				RegistrationState: snapshot.RegistrationState,
			})
		}
	}
	return declarations
}
