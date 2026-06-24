package routemodule

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/app/actionwire"
	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire/configmodule"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
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
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	lifecyclecommands "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/commands"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
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

type Handlers struct {
	Auth       *authapi.Handlers
	Management *coreapi.Handlers
	Tasks      *taskapi.Handlers
	EventsWS   *managementws.EventsHandler
}

type serverDeps struct {
	runtime            configmodule.RuntimeState
	auth               *auth.Manager
	tasks              *tasks.Registry
	plugins            *plugincatalog.Catalog
	pluginInstaller    plugins.InstallCoordinator
	pluginUninstaller  plugins.UninstallCoordinator
	pluginRepository   plugins.DesiredStateRepository
	pluginLifecycle    *pluginservice.Controller
	renderer           *renderservice.Service
	configHandler      *configapi.Handlers
	authHandler        *authapi.Handlers
	managementHandler  *coreapi.Handlers
	governanceHandler  *governanceapi.Handlers
	taskHandler        *taskapi.Handlers
	logHandler         *logapi.Handlers
	renderHandler      *renderapi.Handlers
	systemHandler      *systemapi.Handlers
	protocolHandler    *protocolapi.Handlers
	thirdPartyHandler  *thirdpartyapi.ThirdPartyHandlers
	bilibiliHandler    *bilibiliapi.BilibiliHandlers
	eventsWS           *managementws.EventsHandler
	tasksWS            *managementws.TasksHandler
	logsWS             *managementws.LogsHandler
	consoleWS          *managementws.ConsoleHandler
	pluginWebhooks     *pluginwebhook.Service
	pluginManagementUI *pluginui.Handlers
	metrics            *metrics.Registry
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
	configHandler := configapi.NewHandlers(configService)
	authHandler := authapi.NewHandlers(authapi.Deps{
		Config:        authConfigSource{runtime: runtimeState},
		Auth:          platformState.Auth,
		LoginFailures: platformState.LoginFailures,
	})
	managementHandler := coreapi.NewHandlers(coreapi.Deps{
		Auth:            platformState.Auth,
		System:          services.System,
		RequestShutdown: deps.RequestShutdown,
	})
	governanceHandler := governanceapi.NewHandlersWithService(services.Governance)
	taskHandler := taskapi.NewHandlers(platformState.Tasks, platformState.TaskExecutor, pluginState.PluginInstaller)
	logHandler := logapi.NewHandlers(services.Logs)
	renderHandler := renderapi.NewHandlers(renderer)
	systemHandler := systemapi.NewHandlers(services.System, platformState.Scheduler)
	protocolHandler := protocolapi.NewHandlers(services.Protocol)
	thirdPartyHandler := thirdpartyapi.NewThirdPartyHandlers(services.ThirdParty, deps.ServiceBuild.BilibiliAccountClient, services.ThirdPartyQRLogin, services.BilibiliSource, deps.BilibiliHTTPTransport, thirdpartyapi.WithDouyinUserResolver(services.DouyinBrowser))
	bilibiliHandler := bilibiliapi.NewBilibiliHandlers(services.BilibiliSource, deps.ServiceBuild.BilibiliQRLogin, deps.BilibiliHTTPTransport)
	eventsWS := managementws.NewEventsHandler(eventState.Bridge, pluginState.Plugins, services.Protocol, deps.ServiceBuild.Status, services.GovernanceEvents, services.BilibiliEvents)
	tasksWS := managementws.NewTasksHandler(platformState.Tasks)
	logsWS := managementws.NewLogsHandler(services.Logs)
	consoleWS := managementws.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:      pluginState.Plugins,
		PluginConfig: pluginState.PluginConfig,
		Secrets:      platformState.Secrets,
		NotifyConfigChange: func(ctx context.Context, pluginID string) {
			dispatch := actionwire.ConfigChangedDispatcher(eventState.Dispatcher)
			if dispatch != nil {
				dispatch(ctx, pluginID)
			}
		},
		RefreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			lifecyclecommands.RefreshPluginCommands(pluginState.Plugins, eventState.Dispatcher, pluginID, settings)
		},
	})

	router, server := buildAppHTTPServer(serverDeps{
		runtime:            runtimeState,
		auth:               platformState.Auth,
		tasks:              platformState.Tasks,
		plugins:            pluginState.Plugins,
		pluginInstaller:    pluginState.PluginInstaller,
		pluginUninstaller:  pluginState.PluginUninstaller,
		pluginRepository:   pluginState.PluginRepository,
		pluginLifecycle:    services.PluginLifecycle,
		renderer:           renderer,
		configHandler:      configHandler,
		authHandler:        authHandler,
		managementHandler:  managementHandler,
		governanceHandler:  governanceHandler,
		taskHandler:        taskHandler,
		logHandler:         logHandler,
		renderHandler:      renderHandler,
		systemHandler:      systemHandler,
		protocolHandler:    protocolHandler,
		thirdPartyHandler:  thirdPartyHandler,
		bilibiliHandler:    bilibiliHandler,
		eventsWS:           eventsWS,
		tasksWS:            tasksWS,
		logsWS:             logsWS,
		consoleWS:          consoleWS,
		pluginWebhooks:     services.PluginWebhooks,
		pluginManagementUI: pluginManagementUIHandler,
		metrics:            deps.Metrics,
	})
	return State{
		Router: router,
		Server: server,
		Handlers: Handlers{
			Auth:       authHandler,
			Management: managementHandler,
			Tasks:      taskHandler,
			EventsWS:   eventsWS,
		},
	}
}

func buildAppHTTPServer(deps serverDeps) (http.Handler, *http.Server) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(deps.runtime.RuntimeLogger(), httpapi.WithRequestObserver(metrics.NewHTTPObserver(deps.metrics))))

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

type authConfigSource struct {
	runtime configmodule.RuntimeState
}

func (s authConfigSource) AuthConfig() authapi.Config {
	if s.runtime == nil {
		return authapi.Config{}
	}
	cfg := s.runtime.CurrentConfig()
	return authapi.Config{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  authapi.LoginFailureLimit(cfg),
		LoginFailureWindow: authapi.LoginFailureWindow(cfg),
	}
}
