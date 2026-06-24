package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func (a *App) Run(ctx context.Context) error {
	supervisor := newRunSupervisor(ctx)
	runCtx := supervisor.Context()
	a.setRunCancel(supervisor.Cancel)
	defer a.clearRunCancel()

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
	storage.StartSnapshotLoop(runCtx, a.platform.Storage, a.state.Logger)
	a.eventStack.Adapter.Start(runCtx)
	a.platform.Scheduler.Start(runCtx)
	if a.services.BilibiliSource != nil {
		supervisor.Go(func(ctx context.Context) error {
			a.services.BilibiliSource.Start(ctx)
			return nil
		})
	}

	supervisor.GoCritical(func(context.Context) error {
		a.state.Logger.Info("http server starting", "component", "app", "listen_addr", a.process.server.Addr)
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
	a.state.Logger.Info("http server shutting down", "component", "app", "listen_addr", a.process.server.Addr)
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

func (s *appRuntimeState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}
