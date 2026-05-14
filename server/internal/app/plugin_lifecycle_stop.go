package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) stopAndResetPlugin(pluginID string) {
	if c == nil {
		return
	}
	c.stopPlugin(context.Background(), pluginID, true)
}

func (c *pluginLifecycleController) stopPluginAsync(pluginID string, remove bool) {
	if c == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.stopPlugin(ctx, pluginID, remove)
}

func (c *pluginLifecycleController) stopPlugin(ctx context.Context, pluginID string, remove bool) {
	if c == nil || c.runtimes == nil {
		return
	}

	c.clearBotIdentity(pluginID)
	c.dispatcher.Deregister(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
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
		c.runtimes.Delete(pluginID)
	}
	if c.webhooks != nil {
		c.webhooks.DeletePlugin(pluginID)
	}
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
}

func (c *pluginLifecycleController) handleCrash(pluginID string, crashCount int, _ string) {
	if c == nil {
		return
	}
	if c.dispatcher != nil {
		c.dispatcher.Deregister(pluginID)
	}
	c.clearBotIdentity(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	maxRetries := runtime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		manager.SetDeadLetterState()
		runtimeSnapshot := manager.Snapshot()
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateDeadLetter))
		if c.plugins != nil && runtimeSnapshot.EnteredDeadLetterAt != nil {
			_, _ = c.plugins.SetDeadLetterSnapshot(pluginID, plugins.DeadLetterSnapshot{
				EnteredAt:        *runtimeSnapshot.EnteredDeadLetterAt,
				CrashCount:       runtimeSnapshot.CrashCount,
				LastErrorCode:    runtimeSnapshot.LastErrorCode,
				LastErrorMessage: runtimeSnapshot.LastErrorMessage,
			})
		}
		if c.webhooks != nil {
			c.webhooks.DeletePlugin(pluginID)
		}
		c.state.Logger.Warn(
			"plugin entered dead_letter after repeated crashes",
			"component", "app",
			"plugin_id", pluginID,
			"crash_count", crashCount,
			"max_retries", maxRetries,
		)
		return
	}

	cfg := c.state.Config.Runtime
	delay := runtime.CrashBackoff(crashCount, cfg.CrashBackoffInitialSeconds, cfg.CrashBackoffMaxSeconds)
	nextRetry := time.Now().Add(delay)

	manager.SetBackoffState(nextRetry)
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateBackoff))

	c.state.Logger.Info(
		"plugin runtime entering backoff before restart",
		"component", "app",
		"plugin_id", pluginID,
		"crash_count", crashCount,
		"backoff_seconds", int(delay.Seconds()),
	)

	go c.backoffRestart(pluginID, delay)
}

func (c *pluginLifecycleController) backoffRestart(pluginID string, delay time.Duration) {
	if c == nil {
		return
	}

	time.Sleep(delay)

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	if manager.Snapshot().State != runtime.StateBackoff {
		return
	}

	botID := c.currentBotID()

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.state.Config.Runtime))
	defer cancel()

	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
}

func (c *pluginLifecycleController) currentBotID() string {
	if c == nil || c.adapter == nil {
		return ""
	}
	snapshot := c.adapter.Snapshot()
	if snapshot.State != adapter.StateConnected {
		return ""
	}
	return strings.TrimSpace(snapshot.BotID)
}

func (c *pluginLifecycleController) CurrentBotID() string {
	return c.currentBotID()
}

func (c *pluginLifecycleController) logLifecycleWarn(message, pluginID string, err error) {
	if c == nil || c.state == nil || c.state.Logger == nil || err == nil {
		return
	}

	c.state.Logger.Warn(
		message,
		"component", "app",
		"plugin_id", pluginID,
		"err", err.Error(),
	)
}

func (c *pluginLifecycleController) disablePluginForPermissionLoss(ctx context.Context, pluginID string) {
	if c == nil {
		return
	}

	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "disabled"); err != nil {
		c.logLifecycleWarn("persist disabled desired_state after permission rejection", pluginID, err)
	}
	if _, err := c.plugins.SetDesiredState(pluginID, "disabled"); err != nil && !errors.Is(err, plugins.ErrPluginNotFound) {
		c.logLifecycleWarn("set disabled desired_state after permission rejection", pluginID, err)
	}
	c.stopPlugin(ctx, pluginID, true)
	c.reconcileRecoverySummaryBestEffort("plugin.permission_disable")
}
