package pluginstack

import (
	"context"
	"log/slog"
	"path/filepath"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type Deps struct {
	Context   context.Context
	Config    config.Config
	Logger    *slog.Logger
	Discovery runtimepaths.PluginDiscoverySpec
	Validator *schema.Validator
	Catalog   *plugincatalog.Catalog
	Tasks     *tasks.Registry
	Platform  appplatform.State
}

type State struct {
	Plugins           *plugincatalog.Catalog
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	PluginRepository  plugins.DesiredStateRepository
	PluginConfig      pluginconfig.Repository
	PluginFiles       *pluginfile.Service
	PluginKV          pluginkv.Repository
	Webhooks          *pluginwebhook.Registry
	PluginLogLimiter  *localaction.PluginLogLimiter
	RefreshManifest   func(context.Context, string) (plugins.Snapshot, error)
}

func Build(deps Deps) (State, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}

	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(deps.Platform)
	if err != nil {
		return State{}, err
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(deps.Platform.Storage.Path), "plugins"))

	if err := hydratePluginCatalog(ctx, deps.Catalog, pluginRepository, pluginConfigRepository); err != nil {
		return State{}, err
	}
	runtimepaths.CleanupOrphanedInstallDirs(deps.Logger, deps.Discovery.Roots)

	pluginInstallService, pluginUninstallService, err := buildPluginMutationServices(deps, pluginRepository)
	if err != nil {
		return State{}, err
	}

	return State{
		Plugins:           deps.Catalog,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		PluginRepository:  pluginRepository,
		PluginConfig:      pluginConfigRepository,
		PluginFiles:       pluginFileService,
		PluginKV:          pluginKVRepository,
		Webhooks:          webhookRegistry,
		PluginLogLimiter:  localaction.NewPluginLogLimiter(deps.Config),
		RefreshManifest:   buildManifestRefresh(deps, pluginRepository, pluginConfigRepository),
	}, nil
}

func buildManifestRefresh(
	deps Deps,
	pluginRepository plugins.DesiredStateRepository,
	pluginConfigRepository pluginconfig.Repository,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginservice.RefreshPluginManifest(ctx, deps.Catalog, pluginConfigRepository, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: deps.Validator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Logger,
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
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
