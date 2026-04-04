package app

import (
	"context"
	"errors"
	"fmt"
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

func (a *App) Handler() http.Handler {
	return a.router
}

func (a *App) Close() error {
	var errs []error
	if a != nil && a.Runtimes != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.Runtimes.StopAll(stopCtx); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
		cancel()
		a.Runtimes = nil
	}
	if a != nil && a.Dispatcher != nil {
		a.Dispatcher.Close()
		a.Dispatcher = nil
	}
	if a != nil && a.PluginInstaller != nil {
		if err := a.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.PluginInstaller = nil
	}
	if a != nil && a.taskExecutor != nil {
		if err := a.taskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
		a.taskExecutor = nil
	}
	if a != nil && a.PluginUninstaller != nil {
		if closer, ok := a.PluginUninstaller.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
			}
		}
		a.PluginUninstaller = nil
	}
	if a != nil && a.renderer != nil {
		if err := a.renderer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close render service: %w", err))
		}
		a.renderer = nil
	}
	if err := a.closeStorage(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	a.setRunCancel(cancel)
	defer a.clearRunCancel()

	a.autoPrepareRuntimeEnvironments(runCtx)
	if err := runCtx.Err(); err != nil {
		closeErr := a.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	a.Adapter.Start(runCtx)
	a.Scheduler.Start(runCtx)

	go func() {
		a.Logger.Info("http server starting", "component", "app", "listen_addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-runCtx.Done():
		a.shuttingDown.Store(true)
		a.Logger.Info("http server shutting down", "component", "app", "listen_addr", a.server.Addr)
		a.Scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if err := a.Runtimes.StopAll(runtimeStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop runtime managers: %w", err)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if err := a.Adapter.Stop(adapterStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop adapter shell: %w", err)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return a.Close()
	case err := <-errCh:
		a.Scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if stopErr := a.Runtimes.StopAll(runtimeStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop runtime managers after http server error: %w", stopErr)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if stopErr := a.Adapter.Stop(adapterStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop adapter shell after http server error: %w", stopErr)
		}

		closeErr := a.Close()

		if err != nil {
			if closeErr != nil {
				return errors.Join(fmt.Errorf("listen on %s: %w", a.server.Addr, err), closeErr)
			}
			return fmt.Errorf("listen on %s: %w", a.server.Addr, err)
		}
		return closeErr
	}
}

func (a *App) setRunCancel(cancel context.CancelFunc) {
	a.runCancelMu.Lock()
	defer a.runCancelMu.Unlock()
	a.runCancel = cancel
}

func (a *App) clearRunCancel() {
	a.runCancelMu.Lock()
	defer a.runCancelMu.Unlock()
	a.runCancel = nil
}

func (a *App) requestShutdown() {
	if a == nil {
		return
	}

	a.shuttingDown.Store(true)
	a.shutdownOnce.Do(func() {
		a.runCancelMu.Lock()
		cancel := a.runCancel
		a.runCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}

func (a *App) closeStorage() error {
	if a == nil || a.Storage == nil {
		return nil
	}

	if err := a.Storage.Close(); err != nil {
		return fmt.Errorf("close sqlite store: %w", err)
	}

	a.Storage = nil
	return nil
}
