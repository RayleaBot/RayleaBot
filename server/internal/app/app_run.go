package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	a.setRunCancel(cancel)
	defer a.clearRunCancel()

	a.services.system.autoPrepareRuntimeEnvironments(runCtx)
	if err := runCtx.Err(); err != nil {
		closeErr := a.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if a.services.pluginLifecycle != nil {
		go a.services.pluginLifecycle.reconcileRuntime(runCtx, a.services.pluginLifecycle.currentBotID())
	}
	a.startSQLiteSnapshotLoop(runCtx)
	a.pluginStack.Adapter.Start(runCtx)
	a.platform.Scheduler.Start(runCtx)
	if a.services.bilibiliSource != nil {
		go a.services.bilibiliSource.Start(runCtx)
	}

	go func() {
		a.state.Logger.Info("http server starting", "component", "app", "listen_addr", a.process.server.Addr)
		if err := a.process.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-runCtx.Done():
		return a.shutdownFromContext()
	case err := <-errCh:
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
