package routemodule

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	managementrouter "github.com/RayleaBot/RayleaBot/server/internal/management/router"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
)

type Handlers struct {
	Auth       *authapi.Handlers
	Management *coreapi.Handlers
	Tasks      *taskapi.Handlers
	EventsWS   *managementws.EventsHandler
}

type managementRouteState struct {
	RouterDeps  managementrouter.Deps
	RequireAuth func(http.Handler) http.Handler
	Handlers    Handlers
}

type managementUIModule interface {
	managementrouter.PublicRouteModule
	managementrouter.ProtectedRouteModule
}

func buildManagementRoutes(deps Deps, configService configapi.Service, pluginManagementUI managementUIModule) managementRouteState {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	services := deps.ServiceBuild.Services

	authHandler := authapi.NewModule(authapi.Deps{
		Config:        authConfigSource{source: runtimeState},
		Auth:          platformState.Auth,
		LoginFailures: platformState.LoginFailures,
	})
	managementHandler := coreapi.NewModule(coreapi.Deps{
		Auth:            platformState.Auth,
		System:          services.System,
		RequestShutdown: deps.RequestShutdown,
	})
	governanceHandler := governanceapi.NewModule(governanceapi.ModuleDeps{Service: services.Governance})
	taskHandler := taskapi.NewModule(taskapi.ModuleDeps{
		Tasks:           platformState.Tasks,
		TaskExecutor:    platformState.TaskExecutor,
		PluginInstaller: pluginState.PluginInstaller,
	})
	logHandler := logapi.NewModule(logapi.ModuleDeps{Logs: services.Logs})
	renderHandler := renderapi.NewModule(renderapi.ModuleDeps{Renderer: deps.Renderer})
	systemModule := systemapi.NewModule(systemapi.ModuleDeps{
		System:    services.System,
		Scheduler: platformState.Scheduler,
		Metrics:   deps.Metrics.HTTPHandler(),
	})
	protocolHandler := protocolapi.NewModule(protocolapi.ModuleDeps{Protocol: services.Protocol})
	thirdPartyHandler := thirdpartyapi.NewModule(thirdpartyapi.ModuleDeps{
		Accounts:         services.ThirdParty,
		AccountValidator: deps.ServiceBuild.ThirdPartyAccountValidator,
		QRLogin:          services.ThirdPartyQRLogin,
	})
	eventsWS := managementws.NewEventsHandler(eventState.Bridge, pluginState.Plugins, services.Protocol, deps.ServiceBuild.Status, services.GovernanceEvents)
	tasksWS := managementws.NewTasksHandler(platformState.Tasks)
	logsWS := managementws.NewLogsHandler(services.Logs)
	consoleWS := managementws.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	configHandler := configapi.NewModule(configapi.ModuleDeps{Config: configService})
	pluginModule := pluginapi.NewModule(pluginapi.RouteDeps{
		Catalog:      pluginState.Plugins,
		TaskRegistry: platformState.Tasks,
		Repository:   pluginState.PluginRepository,
		Installer:    pluginState.PluginInstaller,
		Uninstaller:  pluginState.PluginUninstaller,
		Lifecycle:    services.PluginLifecycle,
	})

	handlers := Handlers{
		Auth:       authHandler,
		Management: managementHandler,
		Tasks:      taskHandler,
		EventsWS:   eventsWS,
	}

	return managementRouteState{
		Handlers:    handlers,
		RequireAuth: authhttp.RequireAuth(platformState.Auth),
		RouterDeps: managementrouter.Deps{
			RepoRoot: runtimeState.RepoRoot(),
			Readiness: func() health.ReadinessReport {
				return systemModule.Handlers.CurrentReadiness()
			},
			PublicRoutes: []managementrouter.PublicRouteModule{
				authHandler,
				managementHandler,
				protocolHandler,
				services.PluginWebhooks,
				pluginManagementUI,
			},
			ProtectedRoutes: []managementrouter.ProtectedRouteModule{
				managementHandler,
				configHandler,
				protocolHandler,
				governanceHandler,
				logHandler,
				systemModule,
				renderHandler,
				thirdPartyHandler,
				taskHandler,
				pluginManagementUI,
				managementrouter.ProtectedRouteFunc(func(r chi.Router) {
					r.Get("/ws/events", eventsWS.HandleEventsWebSocket())
					r.Get("/ws/tasks", tasksWS.HandleTasksWebSocket())
					r.Get("/ws/logs", logsWS.HandleLogsWebSocket())
					r.Get("/ws/plugins/{id}/console", consoleWS.HandlePluginConsoleWebSocket())
				}),
				pluginModule,
			},
		},
	}
}

type authConfigSource struct {
	source configSource
}

type configSource interface {
	CurrentConfig() config.Config
}

func (s authConfigSource) AuthConfig() authapi.Config {
	if s.source == nil {
		return authapi.Config{}
	}
	cfg := s.source.CurrentConfig()
	return authapi.Config{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  authapi.LoginFailureLimit(cfg),
		LoginFailureWindow: authapi.LoginFailureWindow(cfg),
	}
}
