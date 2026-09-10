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

func (a *App) closeResources() error {
	var errs []error
	a.requestShutdown()
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
		a.metricsRuntimeGaugeStop = nil
	}
	if a.runtimes != nil {
		if err := a.stopRuntimeManagers(5 * time.Second); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
		a.runtimes = nil
	}
	if err := a.stopAdapter(5 * time.Second); err != nil {
		errs = append(errs, fmt.Errorf("stop adapters: %w", err))
	}
	a.eventStack.Close()
	if a.pluginStack.PluginInstaller != nil {
		if err := a.pluginStack.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.pluginStack.PluginInstaller = nil
	}
	if a.services.ThirdPartyQRLogin != nil {
		a.services.ThirdPartyQRLogin.Close()
		a.services.ThirdPartyQRLogin = nil
	}
	if a.platform.TaskExecutor != nil {
		if err := a.platform.TaskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
		a.platform.TaskExecutor = nil
	}
	if a.pluginStack.PluginUninstaller != nil {
		if err := a.pluginStack.PluginUninstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
		}
		a.pluginStack.PluginUninstaller = nil
	}
	if a.platform.Tasks != nil {
		if err := a.platform.Tasks.Close(); err != nil {
			errs = append(errs, fmt.Errorf("flush task registry: %w", err))
		}
		a.platform.Tasks = nil
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
		a.platform.Storage = nil
	}
	if a.configLifecycleLock != nil {
		if err := a.configLifecycleLock.Close(); err != nil {
			errs = append(errs, fmt.Errorf("release config lifecycle lock: %w", err))
		}
		a.configLifecycleLock = nil
	}
	return errors.Join(errs...)
}

func (a *App) stopRuntimeManagers(timeout time.Duration) error {
	if a.eventStack.Dispatcher != nil {
		a.eventStack.Dispatcher.CancelPending()
		defer a.eventStack.Dispatcher.Close()
	}
	if a.runtimes == nil {
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
	var errs []error
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for id, client := range a.eventStack.QQOfficial {
		if err := client.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errs = append(errs, fmt.Errorf("stop QQ official adapter %s: %w", id, err))
		}
	}
	for id, shell := range a.eventStack.OneBotShells {
		if shell == a.eventStack.Adapter {
			continue
		}
		if err := shell.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errs = append(errs, fmt.Errorf("stop OneBot adapter %s: %w", id, err))
		}
	}
	if a.eventStack.Adapter != nil {
		if err := a.eventStack.Adapter.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errs = append(errs, fmt.Errorf("stop primary adapter: %w", err))
		}
	}
	return errors.Join(errs...)
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
