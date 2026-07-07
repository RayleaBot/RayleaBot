package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
)

type httpHandlers struct {
	Auth       *managementapi.AuthHandlers
	Management *managementapi.CoreHandlers
	EventsWS   *managementapi.EventsHandler
}

type managementRouteState struct {
	RouterDeps  managementapi.RouteDeps
	RequireAuth func(http.Handler) http.Handler
	Handlers    httpHandlers
}

type managementUIModule interface {
	managementapi.PublicRouteModule
	managementapi.ProtectedRouteModule
}

func buildManagementRoutes(deps httpBuildDeps, configService managementapi.ConfigService, pluginManagementUI managementUIModule) managementRouteState {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	services := deps.ServiceBuild.Services

	authHandler := managementapi.NewAuthHandlers(managementapi.AuthDeps{
		Config:        authConfigSource{source: runtimeState},
		Auth:          platformState.Auth,
		LoginFailures: platformState.LoginFailures,
	})
	managementHandler := managementapi.NewCoreHandlers(managementapi.CoreDeps{
		Auth:            platformState.Auth,
		System:          services.System,
		RequestShutdown: deps.RequestShutdown,
	})
	governanceHandler := managementapi.NewGovernanceHandlersWithService(services.Governance)
	logHandler := managementapi.NewLogHandlers(services.Logs)
	renderHandler := managementapi.NewRenderHandlers(deps.Renderer)
	systemHandlers := managementapi.NewSystemHandlers(services.System)
	if platformState.Scheduler != nil {
		systemHandlers = managementapi.NewSystemHandlers(services.System, platformState.Scheduler)
	}
	systemRoutes := managementapi.NewSystemRoutes(systemHandlers, deps.Metrics.HTTPHandler())
	protocolHandler := managementapi.NewProtocolHandlers(services.Protocol)
	thirdPartyHandler := managementapi.NewThirdPartyHandlers(
		services.ThirdParty,
		deps.ServiceBuild.ThirdPartyAccountValidator,
		services.ThirdPartyQRLogin,
	)
	eventsWS := managementapi.NewEventsHandler(eventState.Bridge, pluginState.Plugins, services.Protocol, deps.ServiceBuild.Status, services.GovernanceEvents)
	logsWS := managementapi.NewLogsHandler(services.Logs)
	consoleWS := managementapi.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	configHandler := managementapi.NewConfigHandlers(configService)
	pluginRoutes := managementapi.PluginRouteDeps{
		Catalog:      pluginState.Plugins,
		TaskRegistry: platformState.Tasks,
		Repository:   pluginState.PluginRepository,
		Installer:    pluginState.PluginInstaller,
		Uninstaller:  pluginState.PluginUninstaller,
		Lifecycle:    services.PluginLifecycle,
	}

	handlers := httpHandlers{
		Auth:       authHandler,
		Management: managementHandler,
		EventsWS:   eventsWS,
	}

	return managementRouteState{
		Handlers:    handlers,
		RequireAuth: managementapi.RequireAuth(platformState.Auth),
		RouterDeps: managementapi.RouteDeps{
			RepoRoot: runtimeState.RepoRoot(),
			Readiness: func() health.ReadinessReport {
				return systemHandlers.CurrentReadiness()
			},
			PublicRoutes: []managementapi.PublicRouteModule{
				authHandler,
				managementHandler,
				protocolHandler,
				services.PluginWebhooks,
				pluginManagementUI,
			},
			ProtectedRoutes: []managementapi.ProtectedRouteModule{
				managementHandler,
				configHandler,
				protocolHandler,
				governanceHandler,
				logHandler,
				systemRoutes,
				renderHandler,
				thirdPartyHandler,
				pluginManagementUI,
				managementapi.ProtectedRouteFunc(func(r chi.Router) {
					r.Get("/ws/events", eventsWS.HandleEventsWebSocket())
					r.Get("/ws/logs", logsWS.HandleLogsWebSocket())
					r.Get("/ws/plugins/{id}/console", consoleWS.HandlePluginConsoleWebSocket())
				}),
				pluginRoutes,
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

func (s authConfigSource) AuthConfig() managementapi.AuthConfig {
	if s.source == nil {
		return managementapi.AuthConfig{}
	}
	cfg := s.source.CurrentConfig()
	return managementapi.AuthConfig{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  managementapi.LoginFailureLimit(cfg),
		LoginFailureWindow: managementapi.LoginFailureWindow(cfg),
	}
}
