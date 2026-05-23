package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	refreshManifest  func(context.Context, string) (plugins.Snapshot, error)

	identityMu       sync.Mutex
	identityByPlugin map[string]string
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
		refreshManifest:  deps.refreshManifest,
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

	if runtimeSnapshot, runtimeErr := c.plugins.SetRuntimeState(updated.PluginID, string(runtime.StateStarting)); runtimeErr == nil {
		updated = runtimeSnapshot
	}
	go c.startPluginAsync(updated.PluginID, c.currentBotID())
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

	if c.refreshManifest != nil {
		refreshed, err := c.refreshManifest(ctx, pluginID)
		if err != nil {
			return plugins.Snapshot{}, err
		}
		snapshot = refreshed
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			return plugins.Snapshot{}, plugins.ErrStateConflict
		}
	}

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return plugins.Snapshot{}, err
	}

	updated, err := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err != nil {
		updated = snapshot
	}

	go c.reloadPluginAsync(pluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.reload")
	return updated, nil
}

func (c *pluginLifecycleController) RecoverFromDeadLetter(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}
	if manager.Snapshot().State != runtime.StateDeadLetter {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return plugins.Snapshot{}, err
	}

	// Persist desired_state and update the catalog before mutating the
	// runtime manager. If persistence or catalog updates fail, the manager
	// must stay in dead_letter so a retry can pick the plugin up cleanly;
	// resetting the manager up front would leave the catalog reporting
	// dead_letter while the manager has already moved to stopped, which
	// would cause subsequent recovery attempts to fail with
	// plugin.not_in_dead_letter.
	updated := snapshot
	if snapshot.DesiredState != "enabled" {
		if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
			return plugins.Snapshot{}, err
		}
		if reEnabled, setErr := c.plugins.SetDesiredState(pluginID, "enabled"); setErr == nil {
			updated = reEnabled
		}
	}

	manager.ResetCrashCount()
	manager.SetStopped()

	if startingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting)); runtimeErr == nil {
		updated = startingSnapshot
	}

	go c.startPluginAsync(updated.PluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.dead_letter_recover")
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
	botID := c.currentBotID()
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *pluginLifecycleController) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if c == nil {
		return
	}
	botID := strings.TrimSpace(event.BotID)
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
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

	if err := c.ensurePluginRunning(ctx, pluginID, c.currentBotID()); err != nil {
		c.logLifecycleWarn("ensure runtime before scheduler trigger", pluginID, err)
		return
	}

	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      time.Now().Unix(),
		PayloadFields:  schedulerPayloadFields(job),
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

func schedulerPayloadFields(job scheduler.Job) map[string]any {
	fields := map[string]any{
		"job_id": job.JobID,
	}
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		return fields
	}
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fields
	}
	for key, value := range payload {
		fields[key] = value
	}
	return fields
}

func (c *pluginLifecycleController) broadcastBotIdentityChanged(ctx context.Context, botID string) {
	if c == nil || c.dispatcher == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return
	}
	for _, pluginID := range c.dispatcher.PluginIDs() {
		c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, botID)
	}
}

func (c *pluginLifecycleController) dispatchBotIdentityChangedToPlugin(ctx context.Context, pluginID string, botID string) {
	if c == nil || c.dispatcher == nil {
		return
	}
	pluginID = strings.TrimSpace(pluginID)
	botID = strings.TrimSpace(botID)
	if pluginID == "" || botID == "" {
		return
	}
	if c.botIdentityAlreadySent(pluginID, botID) {
		return
	}

	now := time.Now()
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("onebot11-bot-identity-%d-%s", now.UnixNano(), botID),
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "bot.identity.changed",
		Timestamp:      now.Unix(),
		Target: &runtime.EventTarget{
			Type: "bot",
			ID:   botID,
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"self_id": botID,
				"time":    now.Unix(),
			},
		},
	})
	if result.Outcome == dispatch.OutcomeDelivered {
		c.markBotIdentitySent(pluginID, botID)
	}
}

func (c *pluginLifecycleController) botIdentityAlreadySent(pluginID string, botID string) bool {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	return c.identityByPlugin != nil && c.identityByPlugin[pluginID] == botID
}

func (c *pluginLifecycleController) markBotIdentitySent(pluginID string, botID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string]string)
	}
	c.identityByPlugin[pluginID] = botID
}

func (c *pluginLifecycleController) clearBotIdentity(pluginID string) {
	if c == nil {
		return
	}
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin != nil {
		delete(c.identityByPlugin, pluginID)
	}
}
