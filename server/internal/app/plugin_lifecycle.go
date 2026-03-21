package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
)

type pluginLifecycleController struct {
	app *App
}

func newPluginLifecycleController(app *App) *pluginLifecycleController {
	if app == nil {
		return nil
	}
	return &pluginLifecycleController{app: app}
}

func (c *pluginLifecycleController) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.app == nil || c.app.Plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if missing := missingCapabilities(snapshot.RequiredPermissions, c.app.Config.Auth.AutoGrantCapabilities); len(missing) > 0 {
		return plugins.Snapshot{}, &plugins.PermissionPendingError{
			PluginID:            pluginID,
			MissingCapabilities: missing,
		}
	}

	if err := c.app.persistPluginDesiredState(ctx, pluginID, "enabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.app.Plugins.SetDesiredState(pluginID, "enabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if botID := c.currentBotID(); botID != "" && c.shouldStartRuntimeFor(updated.PluginID) {
		if runtimeSnapshot, runtimeErr := c.app.Plugins.SetRuntimeState(updated.PluginID, string(runtime.StateStarting)); runtimeErr == nil {
			updated = runtimeSnapshot
		}
		go c.startPluginAsync(updated.PluginID, botID)
	}

	return updated, nil
}

func (c *pluginLifecycleController) Disable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.app == nil || c.app.Plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "disabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if err := c.app.persistPluginDesiredState(ctx, pluginID, "disabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.app.Plugins.SetDesiredState(pluginID, "disabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	runtimeSnapshot := c.app.Runtime.Snapshot()
	if runtimeSnapshot.PluginID == pluginID && runtimeSnapshot.State != runtime.StateStopped {
		if stoppingSnapshot, runtimeErr := c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopping)); runtimeErr == nil {
			updated = stoppingSnapshot
		}
		go c.stopPluginAsync(pluginID)
	}

	return updated, nil
}

func (c *pluginLifecycleController) HandleAdapterReady(ctx context.Context) {
	if c == nil || c.app == nil {
		return
	}
	c.reconcileRuntime(ctx, c.currentBotID())
}

func (c *pluginLifecycleController) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if c == nil || c.app == nil {
		return
	}
	c.reconcileRuntime(ctx, strings.TrimSpace(event.BotID))
}

func (c *pluginLifecycleController) reconcileRuntime(ctx context.Context, botID string) {
	if c == nil || c.app == nil {
		return
	}
	if strings.TrimSpace(botID) == "" {
		return
	}
	if !c.shouldStartRuntimeFor("") {
		return
	}

	snapshot, started, err := ensureRuntimeStartedForBot(
		ctx,
		c.app.Runtime,
		c.app.Plugins,
		c.app.repoRoot,
		c.app.Config.Runtime,
		botID,
		c.app.Config.Auth.AutoGrantCapabilities,
	)
	if err != nil {
		c.logLifecycleWarn("plugin runtime reconcile failed", snapshot.PluginID, err)
		return
	}
	if started {
		if _, err := c.app.Plugins.SetRuntimeState(snapshot.PluginID, string(runtime.StateRunning)); err != nil {
			c.logLifecycleWarn("update plugin runtime state after reconcile", snapshot.PluginID, err)
		}
	}
}

func (c *pluginLifecycleController) shouldStartRuntimeFor(pluginID string) bool {
	if c == nil || c.app == nil || c.app.Runtime == nil {
		return false
	}

	snapshot := c.app.Runtime.Snapshot()
	if snapshot.State != runtime.StateStopped {
		return false
	}
	if snapshot.PluginID == "" {
		return true
	}
	return pluginID == "" || snapshot.PluginID == pluginID
}

func (c *pluginLifecycleController) startPluginAsync(pluginID, botID string) {
	if c == nil || c.app == nil || strings.TrimSpace(botID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.app.Config.Runtime))
	defer cancel()

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	if missing := missingCapabilities(snapshot.RequiredPermissions, c.app.Config.Auth.AutoGrantCapabilities); len(missing) > 0 {
		if err := c.app.persistPluginDesiredState(ctx, pluginID, "disabled"); err != nil {
			c.logLifecycleWarn("persist disabled desired_state after permission rejection", pluginID, err)
		}
		if updated, err := c.app.Plugins.SetDesiredState(pluginID, "disabled"); err == nil {
			snapshot = updated
		}
		_, _ = c.app.Plugins.SetRuntimeState(snapshot.PluginID, string(runtime.StateStopped))
		return
	}

	spec, err := runtime.BuildSpec(snapshot, c.app.repoRoot, c.app.Config.Runtime)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin enable", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: botID,
		},
		Capabilities: append([]string(nil), c.app.Config.Auth.AutoGrantCapabilities...),
	}

	if err := c.app.Runtime.Start(ctx, spec, payload); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
}

func (c *pluginLifecycleController) stopPluginAsync(pluginID string) {
	if c == nil || c.app == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.app.Runtime.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		c.logLifecycleWarn("stop plugin runtime after disable", pluginID, err)
	}
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
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

func (a *App) persistPluginDesiredState(ctx context.Context, pluginID, desiredState string) error {
	if a == nil || a.pluginRepository == nil {
		return nil
	}
	return a.pluginRepository.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
}

func missingCapabilities(required []string, granted []string) []string {
	if len(required) == 0 {
		return nil
	}

	missing := make([]string, 0, len(required))
	for _, capability := range required {
		if capability == "" || slices.Contains(granted, capability) {
			continue
		}
		missing = append(missing, capability)
	}
	return missing
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}
