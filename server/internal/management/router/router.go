package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementui "github.com/RayleaBot/RayleaBot/server/internal/management/ui"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type Deps struct {
	RepoRoot              string
	Readiness             func() health.ReadinessReport
	Auth                  *authapi.Handlers
	Management            *coreapi.Handlers
	Governance            *governanceapi.Handlers
	Config                *configapi.Handlers
	Tasks                 *taskapi.Handlers
	Logs                  *logapi.Handlers
	Render                *renderapi.Handlers
	System                *systemapi.Handlers
	Protocol              *protocolapi.Handlers
	ThirdParty            *thirdpartyapi.ThirdPartyHandlers
	Bilibili              *bilibiliapi.BilibiliHandlers
	EventsWS              *managementws.EventsHandler
	TasksWS               *managementws.TasksHandler
	LogsWS                *managementws.LogsHandler
	ConsoleWS             *managementws.ConsoleHandler
	PluginWebhooks        *pluginwebhook.Service
	PluginManagementUI    *pluginui.Handlers
	Metrics               *metrics.Registry
	PluginCatalog         *plugincatalog.Catalog
	TaskRegistry          *tasks.Registry
	PluginRepository      plugins.DesiredStateRepository
	PluginInstaller       plugins.InstallCoordinator
	PluginUninstaller     plugins.UninstallCoordinator
	PluginLifecycle       *pluginservice.Controller
	GrantRepository       plugins.GrantRepository
	AutoGrantCapabilities func() []string
}

func Register(r chi.Router, deps Deps, requireAuth func(http.Handler) http.Handler) {
	registerPublicRoutes(r, deps)
	r.Group(func(protected chi.Router) {
		protected.Use(requireAuth)
		registerProtectedRoutes(protected, deps)
	})
	r.NotFound(managementui.NewManagementUIHandler(deps.RepoRoot))
}
