package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/scheduler"
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

func (c *pluginLifecycleController) validateActivation(ctx context.Context, snapshot plugins.Snapshot) ([]string, error) {
	granted := c.grantedCapabilities(ctx, snapshot.PluginID)
	if missing := missingCapabilities(snapshot.RequiredPermissions, granted); len(missing) > 0 {
		return granted, &plugins.PermissionPendingError{
			PluginID:            snapshot.PluginID,
			MissingCapabilities: missing,
		}
	}

	if c.app != nil && c.app.grantRepository != nil {
		if changed := scopeChangedSinceGrant(ctx, c.app.grantRepository, snapshot); changed {
			return granted, &plugins.PermissionPendingError{
				PluginID:     snapshot.PluginID,
				ScopeChanged: true,
			}
		}
	}

	return granted, nil
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

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		return plugins.Snapshot{}, err
	}

	if err := c.app.persistPluginDesiredState(ctx, pluginID, "enabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.app.Plugins.SetDesiredState(pluginID, "enabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if botID := c.currentBotID(); botID != "" {
		if runtimeSnapshot, runtimeErr := c.app.Plugins.SetRuntimeState(updated.PluginID, string(runtime.StateStarting)); runtimeErr == nil {
			updated = runtimeSnapshot
		}
		go c.startPluginAsync(updated.PluginID, botID)
	}
	c.app.reconcileRecoverySummaryBestEffort("plugin.enable")

	return updated, nil
}

func (c *pluginLifecycleController) Reload(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.app == nil || c.app.Plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return plugins.Snapshot{}, err
	}

	updated, err := c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err != nil {
		updated = snapshot
	}

	botID := c.currentBotID()
	if botID == "" {
		go c.stopPluginAsync(pluginID, true)
		c.app.reconcileRecoverySummaryBestEffort("plugin.reload")
		return updated, nil
	}

	go c.reloadPluginAsync(pluginID, botID)
	c.app.reconcileRecoverySummaryBestEffort("plugin.reload")
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

	if manager, ok := c.app.Runtimes.Get(pluginID); ok {
		switch manager.Snapshot().State {
		case runtime.StateStarting, runtime.StateRunning, runtime.StateStopping:
			if stoppingSnapshot, runtimeErr := c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopping)); runtimeErr == nil {
				updated = stoppingSnapshot
			}
			go c.stopPluginAsync(pluginID, true)
		default:
			c.app.Dispatcher.Deregister(pluginID)
			c.app.Runtimes.Delete(pluginID)
			manager.ResetCrashCount()
			manager.SetStopped()
			if stoppedSnapshot, runtimeErr := c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped)); runtimeErr == nil {
				updated = stoppedSnapshot
			}
		}
	}
	c.app.reconcileRecoverySummaryBestEffort("plugin.disable")

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

func (c *pluginLifecycleController) HandleSchedulerTrigger(ctx context.Context, job scheduler.Job) {
	if c == nil || c.app == nil {
		return
	}

	pluginID := strings.TrimSpace(job.PluginID)
	if pluginID == "" {
		return
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.app.Logger.Warn(
			"scheduler trigger ignored for unavailable plugin",
			"component", "app",
			"plugin_id", pluginID,
			"job_id", job.JobID,
		)
		return
	}

	botID := c.currentBotID()
	if botID == "" {
		c.app.Logger.Warn(
			"scheduler trigger skipped because adapter bot identity is unavailable",
			"component", "app",
			"plugin_id", pluginID,
			"job_id", job.JobID,
		)
		return
	}

	if err := c.ensurePluginRunning(ctx, pluginID, botID); err != nil {
		c.logLifecycleWarn("ensure runtime before scheduler trigger", pluginID, err)
		return
	}

	result := c.app.Dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      time.Now().Unix(),
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.app.Logger.Warn(
			"scheduler trigger was not queued for plugin runtime",
			"component", "app",
			"plugin_id", pluginID,
			"job_id", job.JobID,
			"outcome", string(result.Outcome),
			"error_code", result.ErrorCode,
		)
	}
}
