package lifecycle

import (
	"context"
	"errors"
	"time"

	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (c *Controller) stopAndResetPlugin(pluginID string) {
	if c == nil {
		return
	}
	c.stopPlugin(c.lifecycleContext(), pluginID, true)
}

func (c *Controller) StopAndResetPlugin(pluginID string) {
	c.stopAndResetPlugin(pluginID)
}

func (c *Controller) StopAndResetPluginWithContext(ctx context.Context, pluginID string) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.stopPlugin(ctx, pluginID, true)
}

func (c *Controller) stopPluginAsync(pluginID string, remove bool) {
	if c == nil {
		return
	}

	ctx, cancel := c.lifecycleTimeoutContext(5 * time.Second)
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
