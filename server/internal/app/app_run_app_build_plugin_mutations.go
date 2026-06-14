package app

import (
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugininstall "github.com/RayleaBot/RayleaBot/server/internal/plugins/install"
	pluginrepository "github.com/RayleaBot/RayleaBot/server/internal/plugins/repository"
	pluginuninstall "github.com/RayleaBot/RayleaBot/server/internal/plugins/uninstall"
)

func buildPluginMutationServices(state appBuildState, pluginRepository *pluginrepository.SQLiteRepository) (plugins.InstallCoordinator, plugins.UninstallCoordinator, error) {
	pluginInstallService, err := plugininstall.NewInstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.RepoRoot,
		state.discoverySpec.Roots,
		time.Duration(state.core.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := pluginuninstall.NewUninstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.RepoRoot,
		state.discoverySpec.Roots,
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin uninstall service: %w", err)
	}
	return pluginInstallService, pluginUninstallService, nil
}
