package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

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
