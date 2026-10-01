package app

import (
	"net"
	"net/http"
	"strconv"
	"time"

	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/go-chi/chi/v5"
)

type httpBuildDeps struct {
	Runtime                 configRuntimeState
	Platform                PlatformState
	Plugins                 PluginStackState
	Events                  EventState
	Renderer                *render.Service
	ServiceBuild            serviceBuildResult
	RequestShutdown         func(systemsvc.StopIntent)
	SetupToken              string
	LauncherControlToken    string
	DevelopmentArtifactRoot string
}

type appHTTPState struct {
	Router   http.Handler
	Server   *http.Server
	Handlers httpHandlers
}

type serverDeps struct {
	runtime  configRuntimeState
	renderer *render.Service
	routes   managementRouteState
}

func buildHTTP(deps httpBuildDeps) (appHTTPState, error) {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	renderer := deps.Renderer
	services := deps.ServiceBuild.Services

	configService := newConfigService(configServiceDeps{
		EffectiveTimezone: platformState.Scheduler.Timezone,
		Runtime:           runtimeState,
		Logs:              platformState.Logs,
		LogRepository:     platformState.LogRepository,
		Renderer:          renderer,
		OutboundLimiter:   eventState.OutboundLimiter,
		Protocol:          services.Protocol,
		EventIngress:      services.EventIngress,
		Secrets:           platformState.Secrets,
	})
	pluginManagementUIHandler := managementapi.NewPluginManagementUIHandlers(managementapi.PluginManagementUIDeps{
		Plugins:       pluginState.Plugins,
		Settings:      services.PluginSettings,
		ActionInvoker: services.PluginLifecycle,
	})

	managementRoutes, err := buildManagementRoutes(deps, configService, pluginManagementUIHandler)
	if err != nil {
		return appHTTPState{}, err
	}
	router, server, handlers := buildAppHTTPServer(serverDeps{
		runtime:  runtimeState,
		renderer: renderer,
		routes:   managementRoutes,
	})
	return appHTTPState{
		Router:   router,
		Server:   server,
		Handlers: handlers,
	}, nil
}

func buildAppHTTPServer(deps serverDeps) (http.Handler, *http.Server, httpHandlers) {
	router := chi.NewRouter()
	cfg := deps.runtime.CurrentConfig()
	router.Use(httpapi.WithRequestContext(deps.runtime.RuntimeLogger()))

	managementapi.RegisterRoutes(router, deps.routes.RouterDeps, deps.routes.RequireAuth)
	handlers := deps.routes.Handlers

	listenAddr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	handler := http.Handler(router)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	logConfiguredServer(deps.runtime, deps.renderer, listenAddr)
	return handler, server, handlers
}

func logConfiguredServer(state configRuntimeState, renderer *render.Service, listenAddr string) {
	summary := state.CurrentSummary()
	repoRoot := state.RepoRoot()
	configPath := logpath.Display(repoRoot, summary.ConfigPath)
	schemaPath := logpath.Display(repoRoot, summary.SchemaPath)
	databasePath := logpath.Display(repoRoot, summary.DatabasePath)
	state.RuntimeLogger().Debug(
		"配置已加载",
		"component", "config",
		"config_path", configPath,
		"schema_path", schemaPath,
		"server_host", summary.ServerHost,
		"server_port", summary.ServerPort,
		"database_engine", summary.DatabaseEngine,
		"database_path", databasePath,
		"logging_level", summary.LoggingLevel,
		"super_admin_count", summary.SuperAdminCount,
		"adapter_count", summary.AdapterCount,
	)
	serverURL := httpapi.DisplayServerURL(listenAddr)
	state.RuntimeLogger().Debug(
		"管理地址已配置",
		"component", "app",
		"listen_addr", listenAddr,
		"url", serverURL,
	)
	for _, issue := range renderer.Diagnostics() {
		message := "渲染资源存在问题：" + issue.Summary
		if issue.Remediation != "" {
			message += "；处理建议：" + issue.Remediation
		}
		state.RuntimeLogger().Warn(
			message,
			"component", "render",
			"code", issue.Code,
			"severity", issue.Severity,
			"summary", issue.Summary,
			"remediation", issue.Remediation,
		)
	}
}
