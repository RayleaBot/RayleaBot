package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type Options struct {
	ConfigPath       string
	SchemaPath       string
	AuthOptions      []auth.Option
	PluginRepoRoot   string
	PluginSchemaPath string
	PluginRoots      []plugins.ScanRoot
	RenderRunner     render.Runner
}

type appCore struct {
	Config     config.Config
	Summary    config.Summary
	Logger     *slog.Logger
	LogLevel   *logging.LevelController
	repoRoot   string
	redactText func(string) string
	startedAt  time.Time
}

type appPlatform struct {
	Auth          *auth.Manager
	Storage       *storage.Store
	Secrets       secrets.Store
	Tasks         *tasks.Registry
	taskExecutor  *tasks.Executor
	Scheduler     *scheduler.Engine
	Logs          *logging.Stream
	LogRepository logging.Repository
	Console       *console.Stream
	loginFailures *loginFailureTracker
}

type appPlugins struct {
	Plugins           *plugins.Catalog
	Adapter           *adapter.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	pluginRepository  plugins.DesiredStateRepository
	pluginConfig      pluginconfig.Repository
	pluginFiles       *pluginfile.Service
	pluginKV          pluginkv.Repository
	grantRepository   plugins.GrantRepository
	blacklistRepo     permission.BlacklistRepository
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	webhooks          *pluginwebhook.Registry
	renderer          *render.Service
	pluginLogLimiter  *localaction.PluginLogLimiter
	outboundLimiter   *outbound.MessageRateLimiter
}

type appProcessState struct {
	router       http.Handler
	server       *http.Server
	shuttingDown atomic.Bool
	runCancelMu  sync.Mutex
	runCancel    context.CancelFunc
	shutdownOnce sync.Once
}

type appRuntimeState struct {
	Config     config.Config
	Summary    config.Summary
	Logger     *slog.Logger
	LogLevel   *logging.LevelController
	repoRoot   string
	redactText func(string) string
	startedAt  time.Time

	recoveryMu           sync.RWMutex
	recoverySummary      *recovery.CompatibilitySummary
	startupRuntimeMu     sync.RWMutex
	startupRuntimeStates map[string]startupRuntimeState
}

type App struct {
	state   *appRuntimeState
	process appProcessState

	auth          *auth.Manager
	storage       *storage.Store
	secrets       secrets.Store
	tasks         *tasks.Registry
	taskExecutor  *tasks.Executor
	scheduler     *scheduler.Engine
	logs          *logging.Stream
	logRepository logging.Repository
	console       *console.Stream
	loginFailures *loginFailureTracker

	plugins           *plugins.Catalog
	adapter           *adapter.Shell
	bridge            *bridge.Bridge
	dispatcher        *dispatch.Dispatcher
	runtimes          *runtimeRegistry
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	pluginInstaller   plugins.InstallCoordinator
	pluginUninstaller plugins.UninstallCoordinator
	pluginRepository  plugins.DesiredStateRepository
	pluginConfig      pluginconfig.Repository
	pluginFiles       *pluginfile.Service
	pluginKV          pluginkv.Repository
	grantRepository   plugins.GrantRepository
	blacklistRepo     permission.BlacklistRepository
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	renderer          *render.Service
	webhookRegistry   *pluginwebhook.Registry
	pluginLogLimiter  *localaction.PluginLogLimiter
	outboundLimiter   *outbound.MessageRateLimiter

	localActions     *localaction.Service
	pluginLifecycle  *pluginLifecycleController
	eventIngress     *eventIngressService
	protocol         *protocolService
	pluginWebhooks   *pluginwebhook.Service
	governance       *governance.Service
	governanceEvents *governanceEventService
	logService       *logService
	systemService    *systemService

	authHandler       *authHTTPHandlers
	managementHandler *managementHTTPHandlers
	taskHandler       *taskHTTPHandlers
	eventsWS          *eventsWSHandler

	metrics                 *metrics.Registry
	metricsRuntimeGaugeStop func()
}

func New(options Options) (*App, error) {
	buildState, err := initializeAppBuild(options)
	if err != nil {
		return nil, err
	}

	schedulerTriggers := newSchedulerTriggerProxy()
	platformState, err := buildAppPlatform(buildState, schedulerTriggers.Handle)
	if err != nil {
		return nil, err
	}

	pluginState, err := buildAppPlugins(buildState, platformState, options.RenderRunner)
	if err != nil {
		return nil, err
	}

	state := &appRuntimeState{
		Config:               buildState.core.Config,
		Summary:              buildState.core.Summary,
		Logger:               buildState.core.Logger,
		LogLevel:             buildState.core.LogLevel,
		repoRoot:             buildState.core.repoRoot,
		redactText:           buildState.core.redactText,
		startedAt:            buildState.core.startedAt,
		startupRuntimeStates: newStartupRuntimeStates(nil),
	}
	metricRegistry := metrics.New()
	pluginState.Bridge.SetMetricsObserver(bridgeMetricsAdapter{registry: metricRegistry})
	pluginState.Dispatcher.SetMetricsObserver(dispatchMetricsAdapter{registry: metricRegistry})
	pluginState.Adapter.SetMetricsObserver(adapterMetricsAdapter{registry: metricRegistry})
	platformState.taskExecutor.SetMetricsObserver(taskMetricsAdapter{registry: metricRegistry})
	pluginState.renderer.SetMetricsObserver(renderMetricsAdapter{registry: metricRegistry})
	stopRuntimeStateGauge := startPluginRuntimeStateGaugeRefresh(metricRegistry, pluginState.Plugins)
	logService := newLogService(platformState.Logs, platformState.LogRepository)
	grantView := &pluginGrantView{
		state:           state,
		plugins:         pluginState.Plugins,
		grantRepository: pluginState.grantRepository,
	}
	pluginState.Dispatcher.SetCapabilityChecker(grantView.capabilityGranted)
	governanceEvents := newGovernanceEventService()
	governanceService := governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return state.Config },
		Plugins:        pluginState.Plugins,
		BlacklistRepo:  pluginState.blacklistRepo,
		WhitelistRepo:  pluginState.whitelistRepo,
		WhitelistState: pluginState.whitelistState,
		NotifyChanged:  governanceEvents.PublishChanged,
	})
	localActions := localaction.New(localaction.Deps{
		CurrentConfig:    func() config.Config { return state.Config },
		Logger:           state.Logger,
		RedactText:       state.redactString,
		Grants:           grantView,
		PluginConfig:     pluginState.pluginConfig,
		PluginFiles:      pluginState.pluginFiles,
		PluginKV:         pluginState.pluginKV,
		Secrets:          platformState.Secrets,
		Scheduler:        platformState.Scheduler,
		Dispatcher:       pluginState.Dispatcher,
		Renderer:         pluginState.renderer,
		Adapter:          pluginState.Adapter,
		PluginLogLimiter: pluginState.pluginLogLimiter,
		Governance:       governanceService,
	})
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		applicationRefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
	})
	runtimeOptions := runtime.Options{
		Console:                    platformState.Console,
		RedactText:                 buildState.managementRedact,
		StderrRateLimitBytesPerSec: buildState.core.Config.Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	}
	runtimeRegistry := newRuntimeRegistry(state.Logger, runtimeOptions)
	systemService := newSystemService(systemServiceDeps{
		state:            state,
		auth:             platformState.Auth,
		adapter:          pluginState.Adapter,
		plugins:          pluginState.Plugins,
		runtimes:         runtimeRegistry,
		renderer:         pluginState.renderer,
		pluginRepository: pluginState.pluginRepository,
		taskExecutor:     platformState.taskExecutor,
		logRepository:    platformState.LogRepository,
	})
	serviceStatusService := newServiceStatusService(systemService)
	systemService.statusPublisher = serviceStatusService
	lifecycle := newPluginLifecycleController(pluginLifecycleDeps{
		state:            state,
		plugins:          pluginState.Plugins,
		desiredStateRepo: pluginState.pluginRepository,
		grants:           grantView,
		runtimes:         runtimeRegistry,
		dispatcher:       pluginState.Dispatcher,
		pluginConfig:     pluginState.pluginConfig,
		adapter:          pluginState.Adapter,
		webhooks:         pluginState.webhooks,
		onRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
	})
	menuService := menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return state.Config },
		Plugins:       pluginState.Plugins,
		Renderer:      pluginState.renderer,
		Sender:        pluginState.outboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if pluginState.outboundLimiter == nil {
				return nil
			}
			return pluginState.outboundLimiter.Wait(ctx, request)
		},
		Logger: state.Logger,
	})
	eventIngress := newEventIngressService(eventIngressDeps{
		state:            state,
		plugins:          pluginState.Plugins,
		replyTargets:     pluginState.replyTargets,
		outboundSender:   pluginState.outboundSender,
		outboundLimiter:  pluginState.outboundLimiter,
		renderer:         pluginState.renderer,
		menu:             menuService,
		bridge:           pluginState.Bridge,
		lifecycle:        lifecycle,
		metadataEnricher: pluginState.Adapter,
		whitelistRepo:    pluginState.whitelistRepo,
		whitelistState:   pluginState.whitelistState,
		blacklistRepo:    pluginState.blacklistRepo,
	})
	protocolService := newProtocolService(state, pluginState.Adapter)
	pluginWebhooks := pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return state.Config },
		Logger:        state.Logger,
		Registry:      pluginState.webhooks,
		Secrets:       platformState.Secrets,
		Plugins:       pluginState.Plugins,
		Dispatcher:    pluginState.Dispatcher,
		Runtime:       lifecycle,
		Grants:        grantView,
	})
	pluginWebhooks.SetReplayMetrics(webhookReplayMetricsAdapter{registry: metricRegistry})
	localActions.SetWebhookGateway(pluginWebhooks)

	application := &App{
		state:             state,
		auth:              platformState.Auth,
		storage:           platformState.Storage,
		secrets:           platformState.Secrets,
		tasks:             platformState.Tasks,
		taskExecutor:      platformState.taskExecutor,
		scheduler:         platformState.Scheduler,
		logs:              platformState.Logs,
		logRepository:     platformState.LogRepository,
		console:           platformState.Console,
		loginFailures:     platformState.loginFailures,
		plugins:           pluginState.Plugins,
		adapter:           pluginState.Adapter,
		bridge:            pluginState.Bridge,
		dispatcher:        pluginState.Dispatcher,
		runtimes:          runtimeRegistry,
		replyTargets:      pluginState.replyTargets,
		outboundSender:    pluginState.outboundSender,
		pluginInstaller:   pluginState.PluginInstaller,
		pluginUninstaller: pluginState.PluginUninstaller,
		pluginRepository:  pluginState.pluginRepository,
		pluginConfig:      pluginState.pluginConfig,
		pluginFiles:       pluginState.pluginFiles,
		pluginKV:          pluginState.pluginKV,
		grantRepository:   pluginState.grantRepository,
		blacklistRepo:     pluginState.blacklistRepo,
		whitelistRepo:     pluginState.whitelistRepo,
		whitelistState:    pluginState.whitelistState,
		renderer:          pluginState.renderer,
		webhookRegistry:   pluginState.webhooks,
		pluginLogLimiter:  pluginState.pluginLogLimiter,
		outboundLimiter:   pluginState.outboundLimiter,
		localActions:      localActions,
		pluginLifecycle:   lifecycle,
		eventIngress:      eventIngress,
		protocol:          protocolService,
		pluginWebhooks:    pluginWebhooks,
		governance:              governanceService,
		governanceEvents:        governanceEvents,
		logService:              logService,
		systemService:           systemService,
		metrics:                 metricRegistry,
		metricsRuntimeGaugeStop: stopRuntimeStateGauge,
	}
	systemService.shuttingDown = &application.process.shuttingDown
	systemService.RefreshRecoverySummary()
	schedulerTriggers.Set(lifecycle.HandleSchedulerTrigger)

	if installer, ok := application.pluginInstaller.(interface{ SetAfterSuccess(func(string) error) }); ok {
		installer.SetAfterSuccess(func(string) error {
			if err := syncCatalogRenderTemplates(context.Background(), application.renderer, application.plugins); err != nil {
				return err
			}
			systemService.ReconcileRecoverySummaryBestEffort("plugin.install")
			return nil
		})
	}
	if installer, ok := application.pluginInstaller.(interface {
		SetRenderTemplateValidator(func(plugins.Snapshot) error)
	}); ok {
		installer.SetRenderTemplateValidator(validatePluginRenderTemplates)
	}
	if uninstaller, ok := application.pluginUninstaller.(interface {
		SetStopPlugin(plugins.StopPluginFunc)
		SetAfterSuccess(func(string))
	}); ok {
		uninstaller.SetStopPlugin(lifecycle.stopAndResetPlugin)
		uninstaller.SetAfterSuccess(func(pluginID string) {
			if application.renderer != nil {
				_ = application.renderer.RemovePluginTemplates(context.Background(), pluginID)
			}
			_ = syncCatalogRenderTemplates(context.Background(), application.renderer, application.plugins)
			systemService.ReconcileRecoverySummaryBestEffort("plugin.uninstall")
		})
	}
	if application.runtimes != nil {
		application.runtimes.SetOnCrash(lifecycle.handleCrash)
	}
	if application.adapter != nil {
		application.adapter.SetEventHandler(eventIngress.HandleAdapterEvent)
		application.adapter.SetReadyHandler(eventIngress.HandleAdapterReady)
		application.adapter.SetStateHandler(func(adapter.Snapshot) {
			systemService.publishStatusSnapshot()
			protocolService.PublishSnapshot()
		})
	}

	configHandler := newConfigHTTPHandlers(configHTTPDeps{
		state:            state,
		logs:             platformState.Logs,
		logRepository:    platformState.LogRepository,
		renderer:         pluginState.renderer,
		pluginLogLimiter: pluginState.pluginLogLimiter,
		outboundLimiter:  pluginState.outboundLimiter,
		protocol:         protocolService,
		eventIngress:     eventIngress,
		blacklistRepo:    pluginState.blacklistRepo,
	})
	authHandler := newAuthHTTPHandlers(authHTTPDeps{
		state:         state,
		auth:          platformState.Auth,
		loginFailures: platformState.loginFailures,
	})
	managementHandler := newManagementHTTPHandlers(managementHTTPDeps{
		state:           state,
		auth:            platformState.Auth,
		system:          systemService,
		requestShutdown: application.requestShutdown,
	})
	governanceHandler := governance.NewHandlersWithService(governanceService)
	taskHandler := newTaskHTTPHandlers(platformState.Tasks, platformState.taskExecutor, pluginState.PluginInstaller)
	logHandler := newLogHTTPHandlers(logService)
	renderHandler := newRenderHTTPHandlers(pluginState.renderer, platformState.taskExecutor)
	systemHandler := newSystemHTTPHandlers(systemService, platformState.Scheduler)
	protocolHandler := newProtocolHTTPHandlers(protocolService)
	eventsWS := newEventsWSHandler(pluginState.Bridge, pluginState.Plugins, protocolService, serviceStatusService, governanceEvents)
	tasksWS := newTasksWSHandler(platformState.Tasks)
	logsWS := newLogsWSHandler(logService)
	consoleWS := newConsoleWSHandler(platformState.Console, pluginState.Plugins)
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:            pluginState.Plugins,
		PluginConfig:       pluginState.pluginConfig,
		Secrets:            platformState.Secrets,
		NotifyConfigChange: localActions.DispatchPluginConfigChanged,
		RefreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			applicationRefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
		},
	})

	router, server := buildAppHTTPServer(httpServerDeps{
		state:              state,
		auth:               platformState.Auth,
		tasks:              platformState.Tasks,
		plugins:            pluginState.Plugins,
		logs:               logService,
		console:            platformState.Console,
		pluginInstaller:    pluginState.PluginInstaller,
		pluginUninstaller:  pluginState.PluginUninstaller,
		pluginRepository:   pluginState.pluginRepository,
		grantRepository:    pluginState.grantRepository,
		pluginLifecycle:    lifecycle,
		taskExecutor:       platformState.taskExecutor,
		renderer:           pluginState.renderer,
		loginFailures:      platformState.loginFailures,
		configHandler:      configHandler,
		authHandler:        authHandler,
		managementHandler:  managementHandler,
		governanceHandler:  governanceHandler,
		taskHandler:        taskHandler,
		logHandler:         logHandler,
		renderHandler:      renderHandler,
		systemHandler:      systemHandler,
		protocolHandler:    protocolHandler,
		eventsWS:           eventsWS,
		tasksWS:            tasksWS,
		logsWS:             logsWS,
		consoleWS:          consoleWS,
		pluginWebhooks:     pluginWebhooks,
		pluginManagementUI: pluginManagementUIHandler,
		metrics:            metricRegistry,
	})
	application.process.router = router
	application.process.server = server
	application.authHandler = authHandler
	application.managementHandler = managementHandler
	application.taskHandler = taskHandler
	application.eventsWS = eventsWS
	return application, nil
}
