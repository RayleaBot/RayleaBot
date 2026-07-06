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

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type appProcessState struct {
	router       http.Handler
	server       *http.Server
	shuttingDown atomic.Bool
	runCancelMu  sync.Mutex
	runCancel    context.CancelFunc
	shutdownOnce sync.Once
}

type appRuntimeState struct {
	Config             config.Config
	Summary            config.Summary
	Logger             *slog.Logger
	LogLevel           *logging.LevelController
	repoRoot           string
	redactText         func(string) string
	addRedactionValues func(...string)
	startedAt          time.Time
}

func newAppRuntimeState(buildState appBuildState) *appRuntimeState {
	state := buildState.core
	return &state
}

func (s *appRuntimeState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}

func (s *appRuntimeState) CurrentSummary() config.Summary {
	if s == nil {
		return config.Summary{}
	}
	return s.Summary
}

func (s *appRuntimeState) SetConfig(cfg config.Config) {
	if s != nil {
		s.Config = cfg
	}
}

func (s *appRuntimeState) SetSummary(summary config.Summary) {
	if s != nil {
		s.Summary = summary
	}
}

func (s *appRuntimeState) RuntimeLogger() *slog.Logger {
	if s == nil {
		return nil
	}
	return s.Logger
}

func (s *appRuntimeState) RuntimeLogLevel() *logging.LevelController {
	if s == nil {
		return nil
	}
	return s.LogLevel
}

func (s *appRuntimeState) RepoRoot() string {
	if s == nil {
		return ""
	}
	return s.repoRoot
}

func (s *appRuntimeState) StartedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.startedAt
}

func (s *appRuntimeState) RedactString(value string) string {
	return s.redactString(value)
}

func (s *appRuntimeState) AddRedactionValues(values ...string) {
	if s == nil || s.addRedactionValues == nil {
		return
	}
	s.addRedactionValues(values...)
}

func (a *App) Run(ctx context.Context) error {
	supervisor := newRunSupervisor(ctx)
	runCtx := supervisor.Context()
	a.setRunCancel(supervisor.Cancel)
	defer a.clearRunCancel()

	if a.services.PluginLifecycle != nil {
		a.services.PluginLifecycle.BindLifecycleContext(runCtx)
	}

	a.services.System.AutoPrepareRuntimeEnvironments(runCtx)
	if err := runCtx.Err(); err != nil {
		closeErr := a.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if a.services.PluginLifecycle != nil {
		supervisor.Go(func(ctx context.Context) error {
			a.services.PluginLifecycle.ReconcileRuntime(ctx, a.services.PluginLifecycle.CurrentBotID())
			return nil
		})
	}
	storage.StartSnapshotLoop(runCtx, a.platform.Storage, a.state.Logger, a.state.RepoRoot())
	a.eventStack.Adapter.Start(runCtx)
	a.platform.Scheduler.Start(runCtx)

	supervisor.GoCritical(func(context.Context) error {
		serverURL := httpapi.DisplayServerURL(a.process.server.Addr)
		a.state.Logger.Info("HTTP 服务正在启动，管理地址："+serverURL, "component", "app", "listen_addr", a.process.server.Addr, "url", serverURL)
		if err := a.process.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	select {
	case <-runCtx.Done():
		return a.shutdownFromContext()
	case err := <-supervisor.Errors():
		return a.shutdownAfterServerExit(err)
	}
}

func (a *App) shutdownFromContext() error {
	a.process.shuttingDown.Store(true)
	serverURL := httpapi.DisplayServerURL(a.process.server.Addr)
	a.state.Logger.Info("HTTP 服务正在关闭，管理地址："+serverURL, "component", "app", "listen_addr", a.process.server.Addr, "url", serverURL)
	a.platform.Scheduler.Stop()
	if err := a.stopRuntimeManagers(5 * time.Second); err != nil {
		return fmt.Errorf("stop runtime managers: %w", err)
	}
	if err := a.stopAdapter(5 * time.Second); err != nil {
		return fmt.Errorf("stop adapter shell: %w", err)
	}
	if err := a.shutdownHTTPServer(5 * time.Second); err != nil {
		return err
	}
	return a.Close()
}

func (a *App) shutdownAfterServerExit(serverErr error) error {
	a.platform.Scheduler.Stop()
	if err := a.stopRuntimeManagers(5 * time.Second); err != nil {
		return fmt.Errorf("stop runtime managers after http server error: %w", err)
	}
	if err := a.stopAdapter(5 * time.Second); err != nil {
		return fmt.Errorf("stop adapter shell after http server error: %w", err)
	}

	closeErr := a.Close()
	if serverErr != nil {
		if closeErr != nil {
			return errors.Join(fmt.Errorf("listen on %s: %w", a.process.server.Addr, serverErr), closeErr)
		}
		return fmt.Errorf("listen on %s: %w", a.process.server.Addr, serverErr)
	}
	return closeErr
}

func (a *App) setRunCancel(cancel context.CancelFunc) {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	a.process.runCancel = cancel
}

func (a *App) clearRunCancel() {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	a.process.runCancel = nil
}

func (a *App) requestShutdown() {
	if a == nil {
		return
	}

	a.process.shuttingDown.Store(true)
	a.process.shutdownOnce.Do(func() {
		a.process.runCancelMu.Lock()
		cancel := a.process.runCancel
		a.process.runCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}

func (a *App) Handler() http.Handler {
	return a.process.router
}

type runSupervisor struct {
	ctx    context.Context
	cancel context.CancelFunc
	errCh  chan error
	once   sync.Once
}

func newRunSupervisor(parent context.Context) *runSupervisor {
	ctx, cancel := context.WithCancel(parent)
	return &runSupervisor{
		ctx:    ctx,
		cancel: cancel,
		errCh:  make(chan error, 1),
	}
}

func (s *runSupervisor) Context() context.Context {
	if s == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *runSupervisor) Cancel() {
	if s == nil {
		return
	}
	s.cancel()
}

func (s *runSupervisor) Go(run func(context.Context) error) {
	if s == nil || run == nil {
		return
	}
	go func() {
		if err := run(s.ctx); err != nil {
			s.report(err)
		}
	}()
}

func (s *runSupervisor) GoCritical(run func(context.Context) error) {
	if s == nil || run == nil {
		return
	}
	go func() {
		s.once.Do(func() {
			s.errCh <- run(s.ctx)
		})
	}()
}

func (s *runSupervisor) report(err error) {
	if s == nil || err == nil {
		return
	}
	s.once.Do(func() {
		s.errCh <- err
	})
}

func (s *runSupervisor) Errors() <-chan error {
	if s == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}
	return s.errCh
}

func (s *appRuntimeState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}

func configureAppRuntimeCallbacks(application *App, schedulerTriggers *schedulerTriggerProxy) {
	systemService := application.services.System
	lifecycle := application.services.PluginLifecycle
	eventIngress := application.services.EventIngress
	protocolService := application.services.Protocol

	systemService.BindShutdownFlag(&application.process.shuttingDown)
	systemService.RefreshRecoverySummary()
	schedulerTriggers.Set(lifecycle.HandleSchedulerTrigger)

	if installer, ok := application.pluginStack.PluginInstaller.(interface {
		SetAfterSuccess(func(context.Context, string) error)
	}); ok {
		installer.SetAfterSuccess(func(ctx context.Context, _ string) error {
			if err := syncCatalogRenderTemplates(ctx, application.renderStack.Renderer, application.pluginStack.Plugins); err != nil {
				return err
			}
			systemService.ReconcileRecoverySummaryBestEffort("plugin.install")
			return nil
		})
	}
	if installer, ok := application.pluginStack.PluginInstaller.(interface {
		SetRenderTemplateValidator(func(plugins.Snapshot) error)
	}); ok {
		installer.SetRenderTemplateValidator(validatePluginRenderTemplates)
	}
	if uninstaller, ok := application.pluginStack.PluginUninstaller.(interface {
		SetStopPlugin(plugins.StopPluginFunc)
		SetAfterSuccess(func(context.Context, string))
	}); ok {
		uninstaller.SetStopPlugin(lifecycle.StopAndResetPluginWithContext)
		uninstaller.SetAfterSuccess(func(ctx context.Context, pluginID string) {
			if application.renderStack.Renderer != nil {
				_ = application.renderStack.Renderer.RemovePluginTemplates(ctx, pluginID)
			}
			_ = syncCatalogRenderTemplates(ctx, application.renderStack.Renderer, application.pluginStack.Plugins)
			systemService.ReconcileRecoverySummaryBestEffort("plugin.uninstall")
		})
	}
	if application.runtimes != nil {
		application.runtimes.SetOnCrash(lifecycle.HandleCrash)
	}
	if application.eventStack.Adapter != nil {
		application.eventStack.Adapter.SetEventHandler(eventIngress.HandleAdapterEvent)
		application.eventStack.Adapter.SetReadyHandler(eventIngress.HandleAdapterReady)
		application.eventStack.Adapter.SetStateHandler(func(onebot11.Snapshot) {
			systemService.PublishStatusSnapshot()
			protocolService.PublishSnapshot()
		})
	}
}
