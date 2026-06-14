package service

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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
		if _, err := c.validateActivation(ctx, snapshot); err != nil {
			c.disablePluginForPermissionLoss(ctx, snapshot.PluginID)
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
	case runtime.StateRunning:
		c.registerRuntimeIfNeeded(pluginID, manager)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
		return nil
	case runtime.StateStarting, runtime.StateStopping, runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter:
		return nil
	default:
	}

	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
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

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	manager := c.runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
}

func (c *Controller) startRuntime(ctx context.Context, pluginID, botID string, manager *runtime.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return nil
	}

	granted, err := c.validateActivation(ctx, snapshot)
	if err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return err
	}
	if err := c.seedPluginDefaultConfig(ctx, snapshot); err != nil {
		return err
	}

	spec, payload, err := c.buildStartInputsWithCapabilities(pluginID, botID, granted)
	if err != nil {
		return err
	}

	c.clearBotIdentity(pluginID)
	if err := manager.Start(ctx, spec, payload); err != nil {
		return err
	}

	manager.ResetCrashCount()
	c.registerRuntime(pluginID, snapshot, manager)
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	return nil
}
