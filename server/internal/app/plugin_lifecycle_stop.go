package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) stopAndResetPlugin(pluginID string) {
	if c == nil || c.app == nil {
		return
	}
	c.stopPlugin(context.Background(), pluginID, true)
}

func (c *pluginLifecycleController) stopPluginAsync(pluginID string, remove bool) {
	if c == nil || c.app == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.stopPlugin(ctx, pluginID, remove)
}

func (c *pluginLifecycleController) stopPlugin(ctx context.Context, pluginID string, remove bool) {
	if c == nil || c.app == nil || c.app.Runtimes == nil {
		return
	}

	c.app.Dispatcher.Deregister(pluginID)

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	switch manager.Snapshot().State {
	case runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter, runtime.StateStopped:
		manager.ResetCrashCount()
		manager.SetStopped()
	default:
		if err := manager.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logLifecycleWarn("stop plugin runtime", pluginID, err)
		}
		manager.ResetCrashCount()
	}

	if remove {
		c.app.Runtimes.Delete(pluginID)
	}
	if c.app.webhooks != nil {
		c.app.webhooks.DeletePlugin(pluginID)
	}
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
}

func (c *pluginLifecycleController) handleCrash(pluginID string, crashCount int, _ string) {
	if c == nil || c.app == nil {
		return
	}

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	maxRetries := runtime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		manager.SetDeadLetterState()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateDeadLetter))
		c.app.Logger.Warn(
			"plugin entered dead_letter after repeated crashes",
			"component", "app",
			"plugin_id", pluginID,
			"crash_count", crashCount,
			"max_retries", maxRetries,
		)
		return
	}

	cfg := c.app.Config.Runtime
	delay := runtime.CrashBackoff(crashCount, cfg.CrashBackoffInitialSeconds, cfg.CrashBackoffMaxSeconds)
	nextRetry := time.Now().Add(delay)

	manager.SetBackoffState(nextRetry)
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateBackoff))

	c.app.Logger.Info(
		"plugin runtime entering backoff before restart",
		"component", "app",
		"plugin_id", pluginID,
		"crash_count", crashCount,
		"backoff_seconds", int(delay.Seconds()),
	)

	go c.backoffRestart(pluginID, delay)
}

func (c *pluginLifecycleController) backoffRestart(pluginID string, delay time.Duration) {
	if c == nil || c.app == nil {
		return
	}

	time.Sleep(delay)

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.app.Runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	if manager.Snapshot().State != runtime.StateBackoff {
		return
	}

	botID := c.currentBotID()
	if botID == "" {
		manager.SetDeadLetterState()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateDeadLetter))
		c.app.Logger.Warn(
			"cannot restart plugin: no bot connection",
			"component", "app",
			"plugin_id", pluginID,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.app.Config.Runtime))
	defer cancel()

	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
}

func (c *pluginLifecycleController) currentBotID() string {
	if c == nil || c.app == nil || c.app.Adapter == nil {
		return ""
	}
	return strings.TrimSpace(c.app.Adapter.Snapshot().BotID)
}

func (c *pluginLifecycleController) logLifecycleWarn(message, pluginID string, err error) {
	if c == nil || c.app == nil || c.app.Logger == nil || err == nil {
		return
	}

	c.app.Logger.Warn(
		message,
		"component", "app",
		"plugin_id", pluginID,
		"err", err.Error(),
	)
}

func (c *pluginLifecycleController) disablePluginForPermissionLoss(ctx context.Context, pluginID string) {
	if c == nil || c.app == nil {
		return
	}

	if err := c.app.persistPluginDesiredState(ctx, pluginID, "disabled"); err != nil {
		c.logLifecycleWarn("persist disabled desired_state after permission rejection", pluginID, err)
	}
	if _, err := c.app.Plugins.SetDesiredState(pluginID, "disabled"); err != nil && !errors.Is(err, plugins.ErrPluginNotFound) {
		c.logLifecycleWarn("set disabled desired_state after permission rejection", pluginID, err)
	}
	c.stopPlugin(ctx, pluginID, true)
	c.app.reconcileRecoverySummaryBestEffort("plugin.permission_disable")
}
