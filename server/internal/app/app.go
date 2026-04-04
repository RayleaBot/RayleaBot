package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/command"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/console"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/pluginconfig"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/recovery"
	"rayleabot/server/internal/render"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/scheduler"
	"rayleabot/server/internal/secrets"
	"rayleabot/server/internal/storage"
	"rayleabot/server/internal/tasks"
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
	Auth           *auth.Manager
	Storage        *storage.Store
	Secrets        secrets.Store
	Tasks          *tasks.Registry
	taskExecutor   *tasks.Executor
	Scheduler      *scheduler.Engine
	Logs           *logging.Stream
	LogRepository  logging.Repository
	Console        *console.Stream
	launcherTokens *launcherTokenStore
	loginFailures  *loginFailureTracker
}

type appPlugins struct {
	Plugins           *plugins.Catalog
	Adapter           *adapter.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	Runtimes          *runtimeRegistry
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
	permissionChecker *permission.Checker
	pluginLifecycle   *pluginLifecycleController
	webhooks          *pluginWebhookRegistry
	renderer          *render.Service
	commandParser     *command.Parser
	pluginLogLimiter  *pluginLogLimiter
}

type appProcessState struct {
	recoverySummary      *recovery.CompatibilitySummary
	router               http.Handler
	server               *http.Server
	shuttingDown         atomic.Bool
	runCancelMu          sync.Mutex
	runCancel            context.CancelFunc
	startupRuntimeMu     sync.RWMutex
	startupRuntimeStates map[string]startupRuntimeState
	shutdownOnce         sync.Once
}

type App struct {
	appCore
	appPlatform
	appPlugins
	appProcessState
}

func New(options Options) (*App, error) {
	buildState, err := initializeAppBuild(options)
	if err != nil {
		return nil, err
	}

	var application *App
	platformState, err := buildAppPlatform(buildState, func(ctx context.Context, job scheduler.Job) {
		if application != nil && application.pluginLifecycle != nil {
			application.pluginLifecycle.HandleSchedulerTrigger(ctx, job)
		}
	})
	if err != nil {
		return nil, err
	}

	pluginState, err := buildAppPlugins(buildState, platformState, options.RenderRunner, func(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
		if application == nil {
			return nil, &runtime.Error{
				Code:    "plugin.internal_error",
				Message: "plugin local action executor is not available",
			}
		}
		return application.executeLocalAction(ctx, pluginID, requestID, action)
	})
	if err != nil {
		return nil, err
	}

	application = &App{
		appCore:     buildState.core,
		appPlatform: platformState,
		appPlugins:  pluginState,
		appProcessState: appProcessState{
			startupRuntimeStates: newStartupRuntimeStates(startupRuntimeKinds()),
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)
	application.refreshRecoverySummary()

	if installer, ok := application.PluginInstaller.(interface{ SetAfterSuccess(func(string)) }); ok {
		installer.SetAfterSuccess(func(string) {
			application.reconcileRecoverySummaryBestEffort("plugin.install")
		})
	}
	if uninstaller, ok := application.PluginUninstaller.(interface {
		SetStopPlugin(plugins.StopPluginFunc)
		SetAfterSuccess(func(string))
	}); ok {
		uninstaller.SetStopPlugin(application.pluginLifecycle.stopAndResetPlugin)
		uninstaller.SetAfterSuccess(func(string) {
			application.reconcileRecoverySummaryBestEffort("plugin.uninstall")
		})
	}
	if application.Runtimes != nil {
		application.Runtimes.SetOnCrash(application.pluginLifecycle.handleCrash)
	}
	if application.Adapter != nil {
		application.Adapter.SetEventHandler(application.handleAdapterEvent)
		application.Adapter.SetReadyHandler(application.handleAdapterReady)
	}

	router, server := buildAppHTTPServer(application)
	application.router = router
	application.server = server
	return application, nil
}
