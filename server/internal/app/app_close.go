package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	a.process.closeOnce.Do(func() {
		a.process.closeErr = a.closeResources()
	})
	return a.process.closeErr
}

// closeResources runs once per App (guarded by closeOnce), so fields are not
// cleared after their owner closes.
func (a *App) closeResources() error {
	var errs []error
	a.requestShutdown()
	if a.httpHandlers.EventsWS != nil {
		a.httpHandlers.EventsWS.Close()
	}
	if err := a.shutdownHTTPServer(5 * time.Second); err != nil {
		errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
	}
	a.process.runCancelMu.Lock()
	wait := a.process.runWait
	a.process.runCancelMu.Unlock()
	if wait != nil {
		if err := wait(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.platform.Scheduler != nil {
		a.platform.Scheduler.Stop()
	}
	if a.metricsRuntimeGaugeStop != nil {
		a.metricsRuntimeGaugeStop()
	}
	if a.pluginStack.PluginInstaller != nil {
		if err := a.pluginStack.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
	}
	if a.pluginStack.PluginUninstaller != nil {
		if err := a.pluginStack.PluginUninstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
		}
	}
	if a.services.PluginLifecycle != nil {
		a.services.PluginLifecycle.Close()
	}
	if a.runtimes != nil {
		if err := a.stopRuntimeManagers(5 * time.Second); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
	}
	if err := a.stopAdapter(5 * time.Second); err != nil {
		errs = append(errs, fmt.Errorf("stop adapters: %w", err))
	}
	a.eventStack.Close()

	if a.services.Browser != nil {
		a.services.Browser.CloseAll()
	}
	if a.platform.TaskExecutor != nil {
		if err := a.platform.TaskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
	}

	if a.platform.Tasks != nil {
		if err := a.platform.Tasks.Close(); err != nil {
			errs = append(errs, fmt.Errorf("flush task registry: %w", err))
		}
	}
	if a.renderStack.Renderer != nil {
		if err := a.renderStack.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close render service: %w", err))
		}
	}
	if a.platform.Logs != nil {
		a.platform.Logs.Close()
	}
	if a.platform.Storage != nil {
		if err := a.platform.Storage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sqlite store: %w", err))
		}
	}
	if a.configLifecycleLock != nil {
		if err := a.configLifecycleLock.Close(); err != nil {
			errs = append(errs, fmt.Errorf("release config lifecycle lock: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (a *App) stopRuntimeManagers(timeout time.Duration) error {
	var drainErr error
	if a.eventStack.Dispatcher != nil {
		drainCtx, cancelDrain := context.WithTimeout(context.Background(), timeout)
		drainErr = a.eventStack.Dispatcher.DrainAll(drainCtx)
		cancelDrain()
		defer a.eventStack.Dispatcher.Close()
	}
	if a.runtimes == nil {
		return drainErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stopErr := a.runtimes.StopAll(ctx)
	var projectionErr error
	if a.pluginStack.Plugins != nil {
		for _, plugin := range a.pluginStack.Plugins.List() {
			manager, exists := a.runtimes.Get(plugin.PluginID)
			if !exists {
				continue
			}
			// Stop may time out while the manager still owns a live process.
			// Project its observed state instead of assuming the attempt completed.
			snapshot := manager.Snapshot()
			_, err := a.pluginStack.Plugins.SetRuntimeResult(plugin.PluginID, string(snapshot.State), snapshot.LastErrorCode, snapshot.LastErrorMessage)
			projectionErr = errors.Join(projectionErr, err)
		}
	}
	return errors.Join(drainErr, stopErr, projectionErr)
}

func (a *App) stopAdapter(timeout time.Duration) error {
	if a.services.Protocol == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.services.Protocol.Stop(ctx)
}

func (a *App) shutdownHTTPServer(timeout time.Duration) error {
	if a.process.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.process.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Join(err, a.process.server.Close())
	}
	return nil
}
