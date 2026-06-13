package app

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

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
