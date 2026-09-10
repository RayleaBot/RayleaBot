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
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type appProcessState struct {
	router       http.Handler
	server       *http.Server
	shuttingDown atomic.Bool
	runCancelMu  sync.Mutex
	runCancel    context.CancelFunc
	runWait      func() error
	closeOnce    sync.Once
	closeErr     error
}

type appRuntimeState struct {
	mu      sync.RWMutex
	config  config.Config
	summary config.Summary

	Logger             *slog.Logger
	LogLevel           *logging.LevelController
	repoRoot           string
	redactText         func(string) string
	addRedactionValues func(...string)
	startedAt          time.Time
}

func (s *appRuntimeState) CurrentConfig() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *appRuntimeState) CurrentSummary() config.Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

func (s *appRuntimeState) SetConfig(cfg config.Config) {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
}

func (s *appRuntimeState) SetSummary(summary config.Summary) {
	s.mu.Lock()
	s.summary = summary
	s.mu.Unlock()
}

func (s *appRuntimeState) RuntimeLogger() *slog.Logger {
	return s.Logger
}

func (s *appRuntimeState) RuntimeLogLevel() *logging.LevelController {
	return s.LogLevel
}

func (s *appRuntimeState) RepoRoot() string {
	return s.repoRoot
}

func (s *appRuntimeState) StartedAt() time.Time {
	return s.startedAt
}

func (s *appRuntimeState) RedactString(value string) string {
	return s.redactString(value)
}

func (s *appRuntimeState) AddRedactionValues(values ...string) {
	if s.addRedactionValues == nil {
		return
	}
	s.addRedactionValues(values...)
}

func (a *App) Run(ctx context.Context) error {
	supervisor := newRunSupervisor(ctx)
	defer supervisor.Cancel()
	runCtx := supervisor.Context()
	started := make(chan struct{})
	if !a.setRunSupervisor(supervisor, started) {
		return errors.New("application is already closing")
	}
	defer a.clearRunCancel()

	if a.services.PluginLifecycle != nil {
		a.services.PluginLifecycle.BindLifecycleContext(runCtx)
	}
	if a.services.AccountValidation != nil {
		supervisor.Go(func(ctx context.Context) error {
			if err := a.services.AccountValidation.Run(ctx); err != nil {
				return fmt.Errorf("run account validation: %w", err)
			}
			return nil
		})
	}

	a.services.System.AutoPrepareRuntimeEnvironments(runCtx)
	if err := runCtx.Err(); err != nil {
		close(started)
		return a.Close()
	}
	if a.services.PluginLifecycle != nil {
		supervisor.Go(func(ctx context.Context) error {
			a.services.PluginLifecycle.ReconcileRuntime(ctx)
			return nil
		})
	}
	supervisor.Go(func(ctx context.Context) error {
		storage.RunSnapshotLoop(ctx, a.platform.Storage, a.state.Logger, a.state.RepoRoot())
		return nil
	})
	// Disabled instances run no transports; keeping the supervisor alive lets
	// the instance switch take effect without restarting the application.
	for _, shell := range a.eventStack.OneBotShells {
		shell.Start(runCtx)
	}
	for _, client := range a.eventStack.QQOfficial {
		client.Start(runCtx)
	}
	a.platform.Scheduler.Start(runCtx)

	supervisor.GoCritical(func(context.Context) error {
		serverURL := httpapi.DisplayServerURL(a.process.server.Addr)
		a.state.Logger.Info("服务正在启动，管理地址："+serverURL, "component", "app", "listen_addr", a.process.server.Addr, "url", serverURL)
		if err := a.process.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen on %s: %w", a.process.server.Addr, err)
		}
		return nil
	})

	close(started)
	<-runCtx.Done()
	serverURL := httpapi.DisplayServerURL(a.process.server.Addr)
	a.state.Logger.Info("服务正在关闭", "component", "app", "listen_addr", a.process.server.Addr, "url", serverURL)
	return a.Close()
}

func (a *App) setRunSupervisor(supervisor *runSupervisor, started <-chan struct{}) bool {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	if a.process.shuttingDown.Load() || a.process.runCancel != nil {
		return false
	}
	a.process.runCancel = supervisor.Cancel
	a.process.runWait = func() error {
		// No tasks may be added after Run has finished starting its services.
		<-started
		return supervisor.Wait()
	}
	return true
}

func (a *App) clearRunCancel() {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	a.process.runCancel = nil
	a.process.runWait = nil
}

func (a *App) requestShutdown() {
	a.process.shuttingDown.Store(true)
	a.process.runCancelMu.Lock()
	cancel := a.process.runCancel
	a.process.runCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) Handler() http.Handler {
	return a.process.router
}

type runSupervisor struct {
	ctx     context.Context
	cancel  context.CancelFunc
	errCh   chan error
	errOnce sync.Once
	workers sync.WaitGroup
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
	return s.ctx
}

func (s *runSupervisor) Cancel() {
	s.cancel()
}

func (s *runSupervisor) Go(run func(context.Context) error) {
	if run == nil {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		if err := run(s.ctx); err != nil {
			s.report(err)
		}
	}()
}

func (s *runSupervisor) GoCritical(run func(context.Context) error) {
	if run == nil {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.report(run(s.ctx))
		s.Cancel()
	}()
}

func (s *runSupervisor) report(err error) {
	if err == nil || (s.ctx.Err() != nil && errors.Is(err, s.ctx.Err())) {
		return
	}
	s.errOnce.Do(func() {
		s.errCh <- err
	})
	s.Cancel()
}

func (s *runSupervisor) Wait() error {
	s.workers.Wait()
	select {
	case err := <-s.errCh:
		return err
	default:
		return nil
	}
}

func (s *appRuntimeState) redactString(value string) string {
	if s.redactText == nil {
		return value
	}
	return s.redactText(value)
}

func configureAppRuntimeCallbacks(application *App) {
	systemService := application.services.System
	lifecycle := application.services.PluginLifecycle
	eventIngress := application.services.EventIngress
	protocolService := application.services.Protocol

	systemService.BindShutdownFlag(&application.process.shuttingDown)
	systemService.RefreshRecoverySummary()

	if application.runtimes != nil {
		application.runtimes.SetOnCrash(lifecycle.HandleCrash)
	}
	// Every adapter feeds the same ingress; the OneBot instance the management
	// surface reports on is additionally the one driving snapshot publication.
	for _, shell := range application.eventStack.OneBotShells {
		shell.SetEventHandler(eventIngress.HandleAdapterEvent)
		shell.SetReadyHandler(eventIngress.HandleAdapterReady)
		shell.SetStateHandler(func(onebot11.Snapshot) {
			protocolService.PublishAdaptersSnapshot()
			lifecycle.SyncBotIdentities(context.Background())
		})
	}
	for _, client := range application.eventStack.QQOfficial {
		client.SetEventHandler(eventIngress.HandleAdapterEvent)
		client.SetReadyHandler(eventIngress.HandleAdapterReady)
		// The QQ adapter has no transport snapshot of its own, so its state
		// reaches the management surface through the adapters listing.
		client.SetStateHandler(func() { protocolService.PublishAdaptersSnapshot(); lifecycle.SyncBotIdentities(context.Background()) })
	}
	if application.eventStack.Adapter != nil {
		application.eventStack.Adapter.SetEventHandler(eventIngress.HandleAdapterEvent)
		application.eventStack.Adapter.SetReadyHandler(eventIngress.HandleAdapterReady)
		application.eventStack.Adapter.SetStateHandler(func(onebot11.Snapshot) {
			lifecycle.SyncBotIdentities(context.Background())
			systemService.PublishStatusSnapshot()
			protocolService.PublishSnapshot()
		})
	}
}
