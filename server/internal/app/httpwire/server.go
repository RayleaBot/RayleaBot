package httpwire

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
	managementrouter "github.com/RayleaBot/RayleaBot/server/internal/management/router"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func buildAppHTTPServer(deps serverDeps) (http.Handler, *http.Server) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(deps.runtime.RuntimeLogger()))

	managementrouter.Register(router, managementRouterDeps(deps), authhttp.RequireAuth(deps.auth))

	cfg := deps.runtime.CurrentConfig()
	listenAddr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	logConfiguredServer(deps.runtime, deps.renderer, listenAddr)
	return router, server
}

func managementRouterDeps(deps serverDeps) managementrouter.Deps {
	return managementrouter.Deps{
		RepoRoot: deps.runtime.RepoRoot(),
		Readiness: func() health.ReadinessReport {
			return deps.systemHandler.CurrentReadiness()
		},
		PublicRoutes: []managementrouter.PublicRouteModule{
			deps.authHandler,
			deps.managementHandler,
			deps.protocolHandler,
			deps.pluginWebhooks,
			deps.pluginManagementUI,
		},
		ProtectedRoutes: []managementrouter.ProtectedRouteModule{
			deps.managementHandler,
			deps.configHandler,
			deps.protocolHandler,
			deps.governanceHandler,
			deps.logHandler,
			systemapi.NewRoutes(deps.systemHandler, deps.metrics.HTTPHandler()),
			deps.renderHandler,
			deps.thirdPartyHandler,
			deps.bilibiliHandler,
			deps.taskHandler,
			deps.pluginManagementUI,
			managementrouter.ProtectedRouteFunc(func(r chi.Router) {
				r.Get("/ws/events", deps.eventsWS.HandleEventsWebSocket())
				r.Get("/ws/tasks", deps.tasksWS.HandleTasksWebSocket())
				r.Get("/ws/logs", deps.logsWS.HandleLogsWebSocket())
				r.Get("/ws/plugins/{id}/console", deps.consoleWS.HandlePluginConsoleWebSocket())
			}),
			pluginapi.RouteDeps{
				Catalog:      deps.plugins,
				TaskRegistry: deps.tasks,
				Repository:   deps.pluginRepository,
				Installer:    deps.pluginInstaller,
				Uninstaller:  deps.pluginUninstaller,
				Lifecycle:    deps.pluginLifecycle,
			},
		},
	}
}

func logConfiguredServer(state RuntimeState, renderer *renderservice.Service, listenAddr string) {
	summary := state.CurrentSummary()
	state.RuntimeLogger().Info(
		"configuration loaded",
		"component", "config",
		"config_path", summary.ConfigPath,
		"schema_path", summary.SchemaPath,
		"server_host", summary.ServerHost,
		"server_port", summary.ServerPort,
		"database_engine", summary.DatabaseEngine,
		"database_path", summary.DatabasePath,
		"web_exposure_mode", summary.WebExposureMode,
		"logging_level", summary.LoggingLevel,
		"super_admin_count", summary.SuperAdminCount,
		"onebot_configured", summary.OneBotConfigured,
		"onebot_endpoint", summary.OneBotEndpoint,
	)
	state.RuntimeLogger().Info(
		"http server configured",
		"component", "app",
		"listen_addr", listenAddr,
	)
	for _, issue := range renderer.Diagnostics() {
		state.RuntimeLogger().Warn(
			"render resource issue detected",
			"component", "render",
			"code", issue.Code,
			"severity", issue.Severity,
			"summary", issue.Summary,
			"remediation", issue.Remediation,
		)
	}
}
