package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *App) Close() error {
	var errs []error
	if a != nil && a.metricsRuntimeGaugeStop != nil {
		a.metricsRuntimeGaugeStop()
		a.metricsRuntimeGaugeStop = nil
	}
	if a != nil && a.runtimes != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.runtimes.StopAll(stopCtx); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
		cancel()
		a.runtimes = nil
	}
	if a != nil {
		a.eventStack.Close()
	}
	if a != nil && a.pluginStack.PluginInstaller != nil {
		if err := a.pluginStack.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.pluginStack.PluginInstaller = nil
	}
	if a != nil && a.platform.TaskExecutor != nil {
		if err := a.platform.TaskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
		a.platform.TaskExecutor = nil
	}
	if a != nil && a.pluginStack.PluginUninstaller != nil {
		if closer, ok := a.pluginStack.PluginUninstaller.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
			}
		}
		a.pluginStack.PluginUninstaller = nil
	}
	if a != nil && a.renderStack.Renderer != nil {
		if err := a.renderStack.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close render service: %w", err))
		}
	}
	if a != nil && a.platform.Logs != nil {
		a.platform.Logs.Close()
	}
	if a != nil && a.platform.Storage != nil {
		if err := a.platform.Storage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sqlite store: %w", err))
		}
		a.platform.Storage = nil
	}
	return errors.Join(errs...)
}

func (a *App) stopRuntimeManagers(timeout time.Duration) error {
	if a == nil || a.runtimes == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.runtimes.StopAll(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (a *App) stopAdapter(timeout time.Duration) error {
	if a == nil || a.eventStack.Adapter == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.eventStack.Adapter.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (a *App) shutdownHTTPServer(timeout time.Duration) error {
	if a == nil || a.process.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.process.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
