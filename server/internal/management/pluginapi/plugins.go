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

	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, installer))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, controller, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, controller, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/dead_letter/recover", newDeadLetterRecoverHandler(catalog, controller, grantRepo, autoGrantProvider))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
	router.Get("/api/plugins/{plugin_id}/grants", newListGrantsHandler(catalog, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/grants", newGrantHandler(catalog, grantRepo))
	router.Delete("/api/plugins/{plugin_id}/grants/{capability}", newRevokeGrantHandler(catalog, grantRepo))
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
