package plugins

import (
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(router chi.Router, catalog *Catalog, taskRegistry *tasks.Registry, repo DesiredStateRepository, installer InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	if catalog == nil {
		catalog = NewCatalog(nil)
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
