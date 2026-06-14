package lifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (c *Controller) stopAndResetPlugin(pluginID string) {
	if c == nil {
		return
	}
	c.stopPlugin(context.Background(), pluginID, true)
}

func (c *Controller) StopAndResetPlugin(pluginID string) {
	c.stopAndResetPlugin(pluginID)
}

func (c *Controller) stopPluginAsync(pluginID string, remove bool) {
	if c == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.stopPlugin(ctx, pluginID, remove)
}

func (c *Controller) stopPlugin(ctx context.Context, pluginID string, remove bool) {
	if c == nil || c.runtimes == nil {
		return
	}

	c.clearBotIdentity(pluginID)
	c.dispatcher.Deregister(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
		return
	}

	switch manager.Snapshot().State {
	case runtimemanager.StateBackoff, runtimemanager.StateCrashed, runtimemanager.StateDeadLetter, runtimemanager.StateStopped:
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
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
}

func (c *Controller) disablePluginForPermissionLoss(ctx context.Context, pluginID string) {
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
