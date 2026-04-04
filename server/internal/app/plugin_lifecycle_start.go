package app

import (
	"context"
	"strings"

	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
)

func (c *pluginLifecycleController) reconcileRuntime(ctx context.Context, botID string) {
	if c == nil || c.app == nil || strings.TrimSpace(botID) == "" {
		return
	}

	for _, snapshot := range c.app.Plugins.List() {
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

func (c *pluginLifecycleController) ensurePluginRunning(ctx context.Context, pluginID, botID string) error {
	if c == nil || c.app == nil || c.app.Runtimes == nil {
		return nil
	}

	manager := c.app.Runtimes.GetOrCreate(pluginID)
	switch manager.Snapshot().State {
	case runtime.StateRunning:
		c.registerRuntimeIfNeeded(pluginID, manager)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
		return nil
	case runtime.StateStarting, runtime.StateStopping, runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter:
		return nil
	default:
	}

	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	return c.startRuntime(ctx, pluginID, botID, manager)
}

func (c *pluginLifecycleController) startPluginAsync(pluginID, botID string) {
	if c == nil || c.app == nil || strings.TrimSpace(botID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.app.Config.Runtime))
	defer cancel()

	manager := c.app.Runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
}

func (c *pluginLifecycleController) reloadPluginAsync(pluginID, botID string) {
	if c == nil || c.app == nil || strings.TrimSpace(botID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.app.Config.Runtime))
	defer cancel()

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}
	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return
	}

	current, ok := c.app.Runtimes.Get(pluginID)
	if !ok || current == nil {
		c.startPluginAsync(pluginID, botID)
		return
	}

	switch current.Snapshot().State {
	case runtime.StateStopped:
		c.startPluginAsync(pluginID, botID)
		return
	case runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter:
		current.ResetCrashCount()
		current.SetStopped()
		c.startPluginAsync(pluginID, botID)
		return
	case runtime.StateStarting, runtime.StateStopping:
		return
	}

	spec, payload, err := c.buildStartInputs(ctx, pluginID, botID)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin reload", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	newManager := c.app.Runtimes.NewDetached()
	if err := c.app.Dispatcher.ReloadPlugin(ctx, pluginID, current, newManager, spec, payload, dispatchCommands(snapshot.Commands)); err != nil {
		c.logLifecycleWarn("reload plugin runtime", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
		return
	}

	c.app.Runtimes.Replace(pluginID, newManager)
	newManager.ResetCrashCount()
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
}

func (c *pluginLifecycleController) startRuntime(ctx context.Context, pluginID, botID string, manager *runtime.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return nil
	}

	granted, err := c.validateActivation(ctx, snapshot)
	if err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return nil
	}
	if err := c.seedPluginDefaultConfig(ctx, snapshot); err != nil {
		return err
	}

	spec, payload, err := c.buildStartInputsWithCapabilities(pluginID, botID, granted)
	if err != nil {
		return err
	}

	if err := manager.Start(ctx, spec, payload); err != nil {
		return err
	}

	manager.ResetCrashCount()
	c.registerRuntime(pluginID, snapshot, manager)
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
	return nil
}

func (c *pluginLifecycleController) buildStartInputs(ctx context.Context, pluginID, botID string) (runtime.Spec, runtime.InitPayload, error) {
	return c.buildStartInputsWithCapabilities(pluginID, botID, c.grantedCapabilities(ctx, pluginID))
}

func (c *pluginLifecycleController) buildStartInputsWithCapabilities(pluginID, botID string, capabilities []string) (runtime.Spec, runtime.InitPayload, error) {
	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return runtime.Spec{}, runtime.InitPayload{}, plugins.ErrPluginNotFound
	}

	spec, err := runtime.BuildSpec(snapshot, c.app.repoRoot, c.app.Config.Runtime)
	if err != nil {
		return runtime.Spec{}, runtime.InitPayload{}, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: botID,
		},
		Capabilities: append([]string(nil), capabilities...),
	}
	return spec, payload, nil
}

func (c *pluginLifecycleController) registerRuntimeIfNeeded(pluginID string, manager *runtime.Manager) {
	if c == nil || c.app == nil || c.app.Dispatcher == nil || manager == nil {
		return
	}
	if c.app.Dispatcher.HasPlugin(pluginID) {
		return
	}
	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return
	}
	c.registerRuntime(pluginID, snapshot, manager)
}

func (c *pluginLifecycleController) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *runtime.Manager) {
	if c == nil || c.app == nil || c.app.Dispatcher == nil || manager == nil {
		return
	}
	runtimeSnapshot := manager.Snapshot()
	c.app.Dispatcher.Register(pluginID, manager, runtimeSnapshot.Subscriptions, dispatchCommands(snapshot.Commands))
}
