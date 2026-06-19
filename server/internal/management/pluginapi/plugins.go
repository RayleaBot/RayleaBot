package pluginapi

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

func RegisterPluginRoutes(router chi.Router, catalog plugins.CatalogView, taskRegistry *tasks.Registry, repo plugins.DesiredStateRepository, installer plugins.InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	if catalog == nil {
		catalog = emptyCatalogView{}
	}

	registerPluginReadRoutes(router, catalog, grantRepo, autoGrantProvider)
	registerPluginInstallRoutes(router, catalog, taskRegistry, installer)
	registerPluginLifecycleRoutes(router, catalog, repo, controller, uninstaller, grantRepo, autoGrantProvider)
	registerPluginDeadLetterRoutes(router, catalog, controller, grantRepo, autoGrantProvider)
	registerPluginGrantRoutes(router, catalog, grantRepo, autoGrantProvider)
}

type emptyCatalogView struct{}

func (emptyCatalogView) List() []plugins.Snapshot {
	return nil
}

func (emptyCatalogView) Get(string) (plugins.Snapshot, bool) {
	return plugins.Snapshot{}, false
}

func (emptyCatalogView) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, plugins.ErrPluginNotFound
}
