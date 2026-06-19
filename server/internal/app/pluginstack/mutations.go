package pluginstack

import (
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugininstall "github.com/RayleaBot/RayleaBot/server/internal/plugins/install"
	pluginrepository "github.com/RayleaBot/RayleaBot/server/internal/plugins/repository"
	pluginuninstall "github.com/RayleaBot/RayleaBot/server/internal/plugins/uninstall"
)

func buildPluginMutationServices(deps Deps, pluginRepository *pluginrepository.SQLiteRepository) (plugins.InstallCoordinator, plugins.UninstallCoordinator, error) {
	pluginInstallService, err := plugininstall.NewInstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		time.Duration(deps.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := pluginuninstall.NewUninstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		nil,
	)
	if err != nil {
		closeErr := pluginInstallService.Close()
		return nil, nil, errors.Join(fmt.Errorf("create plugin uninstall service: %w", err), closeErr)
	}
	return pluginInstallService, pluginUninstallService, nil
}
