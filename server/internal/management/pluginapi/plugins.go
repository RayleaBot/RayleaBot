package pluginapi

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

func RegisterPluginRoutes(router chi.Router, catalog plugins.CatalogView, taskRegistry *tasks.Registry, repo plugins.DesiredStateRepository, installer plugins.InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator) {
	if catalog == nil {
		catalog = emptyCatalogView{}
	}

	registerPluginReadRoutes(router, catalog)
	registerPluginInstallRoutes(router, catalog, taskRegistry, installer)
	registerPluginLifecycleRoutes(router, catalog, repo, controller, uninstaller)
	registerPluginDeadLetterRoutes(router, catalog, controller)
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
