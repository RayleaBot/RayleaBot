package lifecycle

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (c *Controller) reconcileRuntime(ctx context.Context, botID string) {
	if c == nil || c.plugins == nil {
		return
	}
	botID = strings.TrimSpace(botID)

	for _, snapshot := range c.plugins.List() {
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
			continue
		}
		if err := c.ensurePluginRunning(ctx, snapshot.PluginID, botID); err != nil {
			c.logLifecycleWarn("plugin runtime reconcile failed", snapshot.PluginID, err)
		}
	}
}

func (c *Controller) ReconcileRuntime(ctx context.Context, botID string) {
	c.reconcileRuntime(ctx, botID)
}

func (c *Controller) ensurePluginRunning(ctx context.Context, pluginID, botID string) error {
	if c == nil || c.runtimes == nil {
		return nil
	}

	manager := c.runtimes.GetOrCreate(pluginID)
	switch manager.Snapshot().State {
	case runtimemanager.StateRunning:
		c.registerRuntimeIfNeeded(pluginID, manager)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateRunning))
		return nil
	case runtimemanager.StateStarting, runtimemanager.StateStopping, runtimemanager.StateBackoff, runtimemanager.StateCrashed, runtimemanager.StateDeadLetter:
		return nil
	default:
	}

	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStarting))
	return c.startRuntime(ctx, pluginID, botID, manager)
}

func (c *Controller) EnsurePluginRunning(ctx context.Context, pluginID, botID string) error {
	return c.ensurePluginRunning(ctx, pluginID, botID)
}

func (c *Controller) startPluginAsync(pluginID, botID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)

	ctx, cancel := c.lifecycleTimeoutContext(runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	manager := c.runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
	}
}

func (c *Controller) startRuntime(ctx context.Context, pluginID, botID string, manager *runtimemanager.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
		return nil
	}

	if err := c.seedPluginDefaultConfig(ctx, snapshot); err != nil {
		return err
	}

	spec, payload, err := c.buildStartInputsWithCapabilities(pluginID, botID, c.declaredCapabilities(snapshot))
	if err != nil {
		return err
	}

	c.clearBotIdentity(pluginID)
	if err := manager.Start(ctx, spec, payload); err != nil {
		return err
	}

	manager.ResetCrashCount()
	c.registerRuntime(pluginID, snapshot, manager)
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateRunning))
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	return nil
}
