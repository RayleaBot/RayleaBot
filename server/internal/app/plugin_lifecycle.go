package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/config"
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
		return updated, nil
	}

	go c.reloadPluginAsync(pluginID, botID)
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

func (c *pluginLifecycleController) stopAndResetPlugin(pluginID string) {
	if c == nil || c.app == nil {
		return
	}
	c.stopPlugin(context.Background(), pluginID, true)
}

func (c *pluginLifecycleController) stopPluginAsync(pluginID string, remove bool) {
	if c == nil || c.app == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.stopPlugin(ctx, pluginID, remove)
}

func (c *pluginLifecycleController) stopPlugin(ctx context.Context, pluginID string, remove bool) {
	if c == nil || c.app == nil || c.app.Runtimes == nil {
		return
	}

	c.app.Dispatcher.Deregister(pluginID)

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
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
		c.app.Runtimes.Delete(pluginID)
	}
	if c.app.webhooks != nil {
		c.app.webhooks.DeletePlugin(pluginID)
	}
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
}

// handleCrash is the CrashCallback wired into each runtime manager. It drives
// the backoff -> restart -> dead_letter cycle using the config-defined backoff
// parameters and a fixed maximum retry count.
func (c *pluginLifecycleController) handleCrash(pluginID string, crashCount int, _ string) {
	if c == nil || c.app == nil {
		return
	}

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	maxRetries := runtime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		manager.SetDeadLetterState()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateDeadLetter))
		c.app.Logger.Warn(
			"plugin entered dead_letter after repeated crashes",
			"component", "app",
			"plugin_id", pluginID,
			"crash_count", crashCount,
			"max_retries", maxRetries,
		)
		return
	}

	cfg := c.app.Config.Runtime
	delay := runtime.CrashBackoff(crashCount, cfg.CrashBackoffInitialSeconds, cfg.CrashBackoffMaxSeconds)
	nextRetry := time.Now().Add(delay)

	manager.SetBackoffState(nextRetry)
	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateBackoff))

	c.app.Logger.Info(
		"plugin runtime entering backoff before restart",
		"component", "app",
		"plugin_id", pluginID,
		"crash_count", crashCount,
		"backoff_seconds", int(delay.Seconds()),
	)

	go c.backoffRestart(pluginID, delay)
}

func (c *pluginLifecycleController) backoffRestart(pluginID string, delay time.Duration) {
	if c == nil || c.app == nil {
		return
	}

	time.Sleep(delay)

	snapshot, ok := c.app.Plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.app.Runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return
	}

	manager, ok := c.app.Runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	if manager.Snapshot().State != runtime.StateBackoff {
		return
	}

	botID := c.currentBotID()
	if botID == "" {
		manager.SetDeadLetterState()
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateDeadLetter))
		c.app.Logger.Warn(
			"cannot restart plugin: no bot connection",
			"component", "app",
			"plugin_id", pluginID,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.app.Config.Runtime))
	defer cancel()

	_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		_, _ = c.app.Plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
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

func (c *pluginLifecycleController) disablePluginForPermissionLoss(ctx context.Context, pluginID string) {
	if c == nil || c.app == nil {
		return
	}

	if err := c.app.persistPluginDesiredState(ctx, pluginID, "disabled"); err != nil {
		c.logLifecycleWarn("persist disabled desired_state after permission rejection", pluginID, err)
	}
	if _, err := c.app.Plugins.SetDesiredState(pluginID, "disabled"); err != nil && !errors.Is(err, plugins.ErrPluginNotFound) {
		c.logLifecycleWarn("set disabled desired_state after permission rejection", pluginID, err)
	}
	c.stopPlugin(ctx, pluginID, true)
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

func dispatchCommands(commands []plugins.Command) []dispatch.CommandDecl {
	items := make([]dispatch.CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, dispatch.CommandDecl{
			Name:       command.Name,
			Aliases:    append([]string(nil), command.Aliases...),
			Permission: command.Permission,
		})
	}
	return items
}

// grantedCapabilities returns the union of auto_grant_capabilities from config
// and per-plugin explicit grants from the database.
func (c *pluginLifecycleController) grantedCapabilities(ctx context.Context, pluginID string) []string {
	auto := append([]string(nil), c.app.Config.Auth.AutoGrantCapabilities...)
	if c.app.grantRepository == nil {
		return auto
	}
	grants, err := c.app.grantRepository.LoadGrants(ctx, pluginID)
	if err != nil {
		return auto
	}
	for _, g := range grants {
		if !slices.Contains(auto, g.Capability) {
			auto = append(auto, g.Capability)
		}
	}
	return auto
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}

// scopeChangedSinceGrant compares the current manifest scope boundaries with
// the scope_json persisted alongside each existing grant. If any grant's stored
// scope differs from the current manifest scope, the plugin needs re-granting.
func scopeChangedSinceGrant(ctx context.Context, repo plugins.GrantRepository, snapshot plugins.Snapshot) bool {
	grants, err := repo.LoadGrants(ctx, snapshot.PluginID)
	if err != nil || len(grants) == 0 {
		return false
	}
	currentScope := plugins.BuildScopeJSON(snapshot)
	for _, g := range grants {
		if g.ScopeJSON != currentScope {
			return true
		}
	}
	return false
}

func (c *pluginLifecycleController) seedPluginDefaultConfig(ctx context.Context, snapshot plugins.Snapshot) error {
	if c == nil || c.app == nil || c.app.pluginConfig == nil || len(snapshot.DefaultConfig) == 0 {
		return nil
	}
	_, err := c.app.pluginConfig.SeedDefaults(ctx, snapshot.PluginID, snapshot.DefaultConfig)
	return err
}
