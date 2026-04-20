package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type pluginLifecycleController struct {
	state            *appRuntimeState
	plugins          *plugins.Catalog
	desiredStateRepo plugins.DesiredStateRepository
	grants           *pluginGrantView
	runtimes         *runtimeRegistry
	dispatcher       *dispatch.Dispatcher
	pluginConfig     pluginconfig.Repository
	adapter          *adapter.Shell
	webhooks         *pluginwebhook.Registry
	onRecoveryChange func(string)
}

func newPluginLifecycleController(deps pluginLifecycleDeps) *pluginLifecycleController {
	return &pluginLifecycleController{
		state:            deps.state,
		plugins:          deps.plugins,
		desiredStateRepo: deps.desiredStateRepo,
		grants:           deps.grants,
		runtimes:         deps.runtimes,
		dispatcher:       deps.dispatcher,
		pluginConfig:     deps.pluginConfig,
		adapter:          deps.adapter,
		webhooks:         deps.webhooks,
		onRecoveryChange: deps.onRecoveryChange,
	}
}

func (c *pluginLifecycleController) validateActivation(ctx context.Context, snapshot plugins.Snapshot) ([]string, error) {
	granted := c.grants.grantedCapabilities(ctx, snapshot.PluginID)
	if missing := missingCapabilities(snapshot.RequiredPermissions, granted); len(missing) > 0 {
		return granted, &plugins.PermissionPendingError{
			PluginID:            snapshot.PluginID,
			MissingCapabilities: missing,
		}
	}

	if c.grants != nil && c.grants.grantRepository != nil {
		if changed := scopeChangedSinceGrant(ctx, c.grants.grantRepository, snapshot); changed {
			return granted, &plugins.PermissionPendingError{
				PluginID:     snapshot.PluginID,
				ScopeChanged: true,
			}
		}
	}

	return granted, nil
}

func (c *pluginLifecycleController) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		return plugins.Snapshot{}, err
	}

	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.plugins.SetDesiredState(pluginID, "enabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if botID := c.currentBotID(); botID != "" {
		if runtimeSnapshot, runtimeErr := c.plugins.SetRuntimeState(updated.PluginID, string(runtime.StateStarting)); runtimeErr == nil {
			updated = runtimeSnapshot
		}
		go c.startPluginAsync(updated.PluginID, botID)
	}
	c.reconcileRecoverySummaryBestEffort("plugin.enable")

	return updated, nil
}

func (c *pluginLifecycleController) Reload(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
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

	updated, err := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err != nil {
		updated = snapshot
	}

	botID := c.currentBotID()
	if botID == "" {
		go c.stopPluginAsync(pluginID, true)
		c.reconcileRecoverySummaryBestEffort("plugin.reload")
		return updated, nil
	}

	go c.reloadPluginAsync(pluginID, botID)
	c.reconcileRecoverySummaryBestEffort("plugin.reload")
	return updated, nil
}

func (c *pluginLifecycleController) Disable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "disabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "disabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.plugins.SetDesiredState(pluginID, "disabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if manager, ok := c.runtimes.Get(pluginID); ok {
		switch manager.Snapshot().State {
		case runtime.StateStarting, runtime.StateRunning, runtime.StateStopping:
			if stoppingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopping)); runtimeErr == nil {
				updated = stoppingSnapshot
			}
			go c.stopPluginAsync(pluginID, true)
		default:
			c.dispatcher.Deregister(pluginID)
			c.runtimes.Delete(pluginID)
			manager.ResetCrashCount()
			manager.SetStopped()
			if stoppedSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped)); runtimeErr == nil {
				updated = stoppedSnapshot
			}
		}
	}
	c.reconcileRecoverySummaryBestEffort("plugin.disable")

	return updated, nil
}

func (c *pluginLifecycleController) HandleAdapterReady(ctx context.Context) {
	if c == nil {
		return
	}
	c.reconcileRuntime(ctx, c.currentBotID())
}

func (c *pluginLifecycleController) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if c == nil {
		return
	}
	c.reconcileRuntime(ctx, strings.TrimSpace(event.BotID))
}

func (c *pluginLifecycleController) HandleSchedulerTrigger(ctx context.Context, job scheduler.Job) {
	if c == nil {
		return
	}

	pluginID := strings.TrimSpace(job.PluginID)
	if pluginID == "" {
		return
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.state.Logger.Warn(
			"scheduler trigger ignored for unavailable plugin",
			"component", "app",
			"plugin_id", pluginID,
			"job_id", job.JobID,
		)
		return
	}

	botID := c.currentBotID()
	if botID == "" {
		c.state.Logger.Warn(
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

	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      time.Now().Unix(),
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.state.Logger.Warn(
			"scheduler trigger was not queued for plugin runtime",
			"component", "app",
			"plugin_id", pluginID,
			"job_id", job.JobID,
			"outcome", string(result.Outcome),
			"error_code", result.ErrorCode,
		)
	}
}
