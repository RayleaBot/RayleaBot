package pluginapi

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type RouteDeps struct {
	Catalog      *plugincatalog.Catalog
	TaskRegistry *tasks.Registry
	Repository   plugins.DesiredStateRepository
	Installer    plugins.InstallCoordinator
	Uninstaller  plugins.UninstallCoordinator
	Lifecycle    *pluginservice.Controller
}

func RegisterProtectedRoutes(router chi.Router, deps RouteDeps) {
	deps.RegisterProtectedRoutes(router)
}

func NewModule(deps RouteDeps) RouteDeps {
	return deps
}

func (deps RouteDeps) RegisterProtectedRoutes(router chi.Router) {
	RegisterPluginRoutes(
		router,
		deps.Catalog,
		deps.TaskRegistry,
		deps.Repository,
		deps.Installer,
		deps.Lifecycle,
		deps.Uninstaller,
	)
}
