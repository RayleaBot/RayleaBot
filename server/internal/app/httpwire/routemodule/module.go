package routemodule

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire/configmodule"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	managementrouter "github.com/RayleaBot/RayleaBot/server/internal/management/router"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type Deps struct {
	Runtime               configmodule.RuntimeState
	Platform              appplatform.State
	Plugins               pluginstack.State
	Events                eventstack.State
	Renderer              *renderservice.Service
	ServiceBuild          servicegraph.BuildResult
	Metrics               *metrics.Registry
	BilibiliHTTPTransport http.RoundTripper
	RequestShutdown       func()
}

type State struct {
	Router   http.Handler
	Server   *http.Server
	Handlers Handlers
}

type serverDeps struct {
	runtime  configmodule.RuntimeState
	renderer *renderservice.Service
	metrics  *metrics.Registry
	routes   managementRouteState
}

func Build(deps Deps) State {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	renderer := deps.Renderer
	services := deps.ServiceBuild.Services

	configService := configmodule.NewService(configmodule.Deps{
		Runtime:          runtimeState,
		Logs:             platformState.Logs,
		LogRepository:    platformState.LogRepository,
		Renderer:         renderer,
		PluginLogLimiter: pluginState.PluginLogLimiter,
		OutboundLimiter:  eventState.OutboundLimiter,
		Protocol:         services.Protocol,
		EventIngress:     services.EventIngress,
		Secrets:          platformState.Secrets,
	})
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:      pluginState.Plugins,
		PluginConfig: pluginState.PluginConfig,
		Secrets:      platformState.Secrets,
		NotifyConfigChange: func(ctx context.Context, pluginID string) {
			dispatch := localaction.ConfigChangedDispatcher(eventState.Dispatcher)
			if dispatch != nil {
				dispatch(ctx, pluginID)
			}
		},
		RefreshCommands: localaction.RefreshCommands(pluginState.Plugins, eventState.Dispatcher),
		ActionInvoker:   services.PluginLifecycle,
	})

	managementRoutes := buildManagementRoutes(deps, configService, pluginManagementUIHandler)
	router, server, handlers := buildAppHTTPServer(serverDeps{
		runtime:  runtimeState,
		renderer: renderer,
		metrics:  deps.Metrics,
		routes:   managementRoutes,
	})
	return State{
		Router:   router,
		Server:   server,
		Handlers: handlers,
	}
}

func buildAppHTTPServer(deps serverDeps) (http.Handler, *http.Server, Handlers) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(deps.runtime.RuntimeLogger(), httpapi.WithRequestObserver(metrics.NewHTTPObserver(deps.metrics))))

	managementrouter.Register(router, deps.routes.RouterDeps, deps.routes.RequireAuth)
	handlers := deps.routes.Handlers

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
	return router, server, handlers
}

func logConfiguredServer(state configmodule.RuntimeState, renderer *renderservice.Service, listenAddr string) {
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
