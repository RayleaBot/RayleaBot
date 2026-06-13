package app

import (
	"context"
	"errors"
	"time"

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
