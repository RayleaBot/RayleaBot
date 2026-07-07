package app

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/go-chi/chi/v5"
)

type httpBuildDeps struct {
	Runtime         configRuntimeState
	Platform        PlatformState
	Plugins         PluginStackState
	Events          EventState
	Renderer        *renderservice.Service
	ServiceBuild    serviceBuildResult
	Metrics         *MetricsRegistry
	RequestShutdown func()
}

type appHTTPState struct {
	Router   http.Handler
	Server   *http.Server
	Handlers httpHandlers
}

type serverDeps struct {
	runtime  configRuntimeState
	renderer *renderservice.Service
	metrics  *MetricsRegistry
	routes   managementRouteState
}

func buildHTTP(deps httpBuildDeps) appHTTPState {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	renderer := deps.Renderer
	services := deps.ServiceBuild.Services

	configService := newConfigService(configServiceDeps{
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
	pluginManagementUIHandler := managementapi.NewPluginManagementUIHandlers(managementapi.PluginManagementUIDeps{
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
	return appHTTPState{
		Router:   router,
		Server:   server,
		Handlers: handlers,
	}
}

func buildAppHTTPServer(deps serverDeps) (http.Handler, *http.Server, httpHandlers) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(deps.runtime.RuntimeLogger(), httpapi.WithRequestObserver(NewHTTPObserver(deps.metrics))))

	managementapi.RegisterRoutes(router, deps.routes.RouterDeps, deps.routes.RequireAuth)
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

func logConfiguredServer(state configRuntimeState, renderer *renderservice.Service, listenAddr string) {
	summary := state.CurrentSummary()
	repoRoot := state.RepoRoot()
	configPath := logpath.Display(repoRoot, summary.ConfigPath)
	schemaPath := logpath.Display(repoRoot, summary.SchemaPath)
	databasePath := logpath.Display(repoRoot, summary.DatabasePath)
	state.RuntimeLogger().Info(
		"配置已加载：配置文件 "+configPath+"，数据库 "+databasePath+"，日志级别 "+summary.LoggingLevel,
		"component", "config",
		"config_path", configPath,
		"schema_path", schemaPath,
		"server_host", summary.ServerHost,
		"server_port", summary.ServerPort,
		"database_engine", summary.DatabaseEngine,
		"database_path", databasePath,
		"web_exposure_mode", summary.WebExposureMode,
		"logging_level", summary.LoggingLevel,
		"super_admin_count", summary.SuperAdminCount,
		"onebot_configured", summary.OneBotConfigured,
		"onebot_endpoint", summary.OneBotEndpoint,
	)
	serverURL := httpapi.DisplayServerURL(listenAddr)
	state.RuntimeLogger().Info(
		"HTTP 服务已配置，管理地址："+serverURL,
		"component", "app",
		"listen_addr", listenAddr,
		"url", serverURL,
	)
	for _, issue := range renderer.Diagnostics() {
		state.RuntimeLogger().Warn(
			"渲染资源存在问题："+issue.Summary,
			"component", "render",
			"code", issue.Code,
			"severity", issue.Severity,
			"summary", issue.Summary,
			"remediation", issue.Remediation,
		)
	}
}
