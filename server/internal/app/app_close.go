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
		if err := a.stopRuntimeManagers(5 * time.Second); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
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
	if a != nil && a.services.ThirdPartyQRLogin != nil {
		a.services.ThirdPartyQRLogin.Close()
		a.services.ThirdPartyQRLogin = nil
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
	if a != nil && a.platform.Tasks != nil {
		if err := a.platform.Tasks.Close(); err != nil {
			errs = append(errs, fmt.Errorf("flush task registry: %w", err))
		}
		a.platform.Tasks = nil
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
	if a != nil && a.configLifecycleLock != nil {
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
	if a.eventStack.Adapter == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for id, client := range a.eventStack.QQOfficial {
		if err := client.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Logger().Warn("QQ 官方机器人适配器停止时出错。",
				"component", "adapter.qqofficial", "adapter_id", id, "error", err.Error())
		}
	}
	for id, shell := range a.eventStack.OneBotShells {
		if shell == a.eventStack.Adapter {
			continue
		}
		if err := shell.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Logger().Warn("OneBot 适配器停止时出错。",
				"component", "adapter.onebot11", "adapter_id", id, "error", err.Error())
		}
	}
	// The primary carries the error: it is the instance the management surface
	// reports on, so a failure to stop it is worth failing shutdown for.
	if err := a.eventStack.Adapter.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (a *App) shutdownHTTPServer(timeout time.Duration) error {
	if a.process.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.process.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
