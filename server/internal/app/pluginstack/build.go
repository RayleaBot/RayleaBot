package pluginstack

import (
	"log/slog"
	"path/filepath"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type Deps struct {
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
}

func Build(deps Deps) (State, error) {
	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(deps.Platform)
	if err != nil {
		return State{}, err
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(deps.Platform.Storage.Path), "plugins"))

	if err := hydratePluginCatalog(deps.Catalog, pluginRepository, pluginConfigRepository); err != nil {
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
	}, nil
}
