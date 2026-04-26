package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *App) Handler() http.Handler {
	return a.process.router
}

func (a *App) Close() error {
	var errs []error
	if a != nil && a.runtimes != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.runtimes.StopAll(stopCtx); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
		cancel()
		a.runtimes = nil
	}
	if a != nil && a.dispatcher != nil {
		a.dispatcher.Close()
		a.dispatcher = nil
	}
	if a != nil && a.pluginInstaller != nil {
		if err := a.pluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.pluginInstaller = nil
	}
	if a != nil && a.taskExecutor != nil {
		if err := a.taskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
		a.taskExecutor = nil
	}
	if a != nil && a.pluginUninstaller != nil {
		if closer, ok := a.pluginUninstaller.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
			}
		}
		a.pluginUninstaller = nil
	}
	if a != nil && a.renderer != nil {
		if err := a.renderer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close render service: %w", err))
		}
		a.renderer = nil
	}
	if a != nil && a.logs != nil {
		a.logs.Close()
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

	a.systemService.autoPrepareRuntimeEnvironments(runCtx)
	if err := runCtx.Err(); err != nil {
		closeErr := a.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if a.pluginLifecycle != nil {
		go a.pluginLifecycle.reconcileRuntime(runCtx, a.pluginLifecycle.currentBotID())
	}
	a.adapter.Start(runCtx)
	a.scheduler.Start(runCtx)

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
		a.process.shuttingDown.Store(true)
		a.state.Logger.Info("http server shutting down", "component", "app", "listen_addr", a.process.server.Addr)
		a.scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if err := a.runtimes.StopAll(runtimeStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop runtime managers: %w", err)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if err := a.adapter.Stop(adapterStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop adapter shell: %w", err)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.process.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return a.Close()
	case err := <-errCh:
		a.scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if stopErr := a.runtimes.StopAll(runtimeStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop runtime managers after http server error: %w", stopErr)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if stopErr := a.adapter.Stop(adapterStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop adapter shell after http server error: %w", stopErr)
		}

		closeErr := a.Close()
		if err != nil {
			if closeErr != nil {
				return errors.Join(fmt.Errorf("listen on %s: %w", a.process.server.Addr, err), closeErr)
			}
			return fmt.Errorf("listen on %s: %w", a.process.server.Addr, err)
		}
		return closeErr
	}
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

func (a *App) closeStorage() error {
	if a == nil || a.storage == nil {
		return nil
	}

	if err := a.storage.Close(); err != nil {
		return fmt.Errorf("close sqlite store: %w", err)
	}

	a.storage = nil
	return nil
}
