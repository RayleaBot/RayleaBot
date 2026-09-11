package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (c *Controller) reconcileRuntime(ctx context.Context) {
	if c.plugins == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	budgetCtx, cancel := context.WithTimeout(ctx, runtimeInitBudget(c.config().Runtime))
	defer cancel()

	for _, snapshot := range c.plugins.List() {
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
			continue
		}
		if err := budgetCtx.Err(); err != nil {
			c.logLifecycleWarn("plugin runtime reconcile skipped after cumulative init budget", snapshot.PluginID, err)
			continue
		}
		if err := c.ensurePluginRunning(budgetCtx, snapshot.PluginID); err != nil {
			c.logLifecycleWarn("plugin runtime reconcile failed", snapshot.PluginID, err)
		}
	}
}

func (c *Controller) ReconcileRuntime(ctx context.Context) {
	c.reconcileRuntime(ctx)
}

func (c *Controller) ensurePluginRunning(ctx context.Context, pluginID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := c.acquireOperation(ctx, pluginID)
	if err != nil {
		return err
	}
	defer release()

	manager := c.runtimes.GetOrCreate(pluginID)
	switch manager.Snapshot().State {
	case pluginruntime.StateRunning:
		if err := c.registerRuntimeIfNeeded(pluginID, manager); err != nil {
			return err
		}
		c.publishRuntimeState(pluginID, string(pluginruntime.StateRunning))
		return nil
	case pluginruntime.StateStarting, pluginruntime.StateStopping, pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter:
		return nil
	default:
	}

	c.publishRuntimeState(pluginID, string(pluginruntime.StateStarting))
	err = c.startRuntimeLocked(ctx, pluginID, manager)
	if err != nil {
		c.publishRuntimeFailure(pluginID, err)
	}
	return err
}

func (c *Controller) EnsurePluginRunning(ctx context.Context, pluginID string) error {
	return c.ensurePluginRunning(ctx, pluginID)
}

func (c *Controller) startPluginAsync(pluginID string) {

	ctx, cancel := c.lifecycleTimeoutContext(runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	if err := c.startRuntime(ctx, pluginID); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		c.publishRuntimeFailure(pluginID, err)
	}
}

func (c *Controller) startRuntime(ctx context.Context, pluginID string) error {
	release, err := c.acquireOperation(ctx, pluginID)
	if err != nil {
		return err
	}
	defer release()
	manager := c.runtimes.GetOrCreate(pluginID)
	if manager.Snapshot().State == pluginruntime.StateRunning {
		return c.registerRuntimeIfNeeded(pluginID, manager)
	}
	return c.startRuntimeLocked(ctx, pluginID, manager)
}

func (c *Controller) startRuntimeLocked(ctx context.Context, pluginID string, manager *pluginruntime.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return nil
	}

	spec, payload, err := c.buildStartInputs(ctx, pluginID)
	if err != nil {
		return err
	}

	c.clearBotIdentity(pluginID)
	if err := manager.Start(ctx, spec, payload); err != nil {
		return err
	}

	manager.ResetCrashCount()
	if err := c.settings.Activate(ctx, pluginID, payload.Config, func() error {
		latest, _ := c.plugins.Get(pluginID)
		return c.registerRuntime(pluginID, latest, manager)
	}); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), max(spec.ShutdownGrace, time.Second))
		defer cancel()
		c.dispatcher.Deregister(pluginID)
		return errors.Join(err, manager.Stop(cleanupCtx))
	}
	c.publishRuntimeState(pluginID, string(pluginruntime.StateRunning))
	c.afterRuntimeRegistered(ctx, pluginID, payload.Bots)
	return nil
}

// StartInstalled waits for initialization before the installer commits a package.
// The installer has already stopped the previous runtime and refreshed the catalog.
func (c *Controller) StartInstalled(ctx context.Context, pluginID string) error {
	ctx, cancel := context.WithTimeout(ctx, runtimeInitTimeout(c.config().Runtime))
	defer cancel()
	if err := c.startRuntime(ctx, pluginID); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		return errors.Join(err, c.StopAndResetPluginWithContext(cleanupCtx, pluginID))
	}
	return nil
}

func (c *Controller) StopAndResetPluginWithContext(ctx context.Context, pluginID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.stopPlugin(ctx, pluginID, true)
}

func (c *Controller) stopPluginAsync(pluginID string, remove bool) {
	// An accepted disable waits for the current operation before starting its
	// shutdown budget. Server shutdown cancels this wait and stops all runtimes.
	release, err := c.acquireOperation(c.lifecycleContext(), pluginID)
	if err != nil {
		return
	}
	defer release()
	if snapshot, ok := c.plugins.Get(pluginID); ok && snapshot.DesiredState == "enabled" {
		return
	}

	ctx, cancel := c.lifecycleTimeoutContext(c.shutdownTimeout)
	defer cancel()
	if err := c.stopPluginLocked(ctx, pluginID, remove); err != nil {
		c.logLifecycleWarn("stop plugin runtime", pluginID, err)
	}
}

func (c *Controller) stopPlugin(ctx context.Context, pluginID string, remove bool) error {
	if c.runtimes == nil {
		return nil
	}
	release, err := c.acquireOperation(ctx, pluginID)
	if err != nil {
		return err
	}
	defer release()
	return c.stopPluginLocked(ctx, pluginID, remove)
}

func (c *Controller) stopPluginLocked(ctx context.Context, pluginID string, remove bool) error {
	c.clearBotIdentity(pluginID)
	var drainErr error
	if drain := c.dispatcher.DrainPlugin(pluginID); drain != nil {
		drainErr = drain.Wait(ctx)
	}
	defer c.dispatcher.Deregister(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return drainErr
	}

	switch manager.Snapshot().State {
	case pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter, pluginruntime.StateStopped:
		manager.ResetCrashCount()
		manager.SetStopped()
	default:
		stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelStop()
		if err := manager.Stop(stopCtx); err != nil {
			// Keep ownership of a runtime whose shutdown failed so later cleanup
			// can retry; an installer must not remove a possibly live executable.
			return errors.Join(drainErr, err)
		}
		manager.ResetCrashCount()
	}

	if remove {
		c.runtimes.Delete(pluginID)
	}
	c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
	return drainErr
}

func (c *Controller) buildStartInputs(ctx context.Context, pluginID string) (pluginruntime.Spec, pluginruntime.InitPayload, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, plugins.ErrPluginNotFound
	}

	cfg := c.config()
	spec, err := pluginruntime.BuildSpec(snapshot, c.repoRoot, cfg.Runtime)
	if err != nil {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, err
	}

	settings, err := c.settings.Read(ctx, pluginID)
	if err != nil {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, err
	}
	payload := pluginruntime.InitPayload{
		Timezone:        c.effectiveTimezone,
		Bots:            c.botIdentities(),
		Config:          settings,
		Permissions:     pluginPermissionNames(snapshot),
		SuperAdmins:     pluginRuntimeSuperAdmins(cfg),
		CommandPrefixes: cfg.CommandPrefixes(),
	}
	return spec, payload, nil
}

func pluginPermissionNames(snapshot plugins.Snapshot) []string {
	items := make([]string, 0, len(snapshot.Permissions))
	for name := range snapshot.Permissions {
		items = append(items, name)
	}
	sort.Strings(items)
	return items
}

func pluginRuntimeSuperAdmins(cfg config.Config) []string {
	source := cfg.Admin.SuperAdmins
	result := make([]string, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, item := range source {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (c *Controller) afterRuntimeRegistered(ctx context.Context, pluginID string, initBots []chatevent.BotIdentity) {
	c.identityMu.Lock()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string][]chatevent.BotIdentity)
	}
	c.identityByPlugin[pluginID] = append([]chatevent.BotIdentity{}, initBots...)
	c.identityMu.Unlock()
	c.dispatchPluginStarted(ctx, pluginID)
	c.SyncBotIdentities(ctx)
}

func (c *Controller) registerRuntimeIfNeeded(pluginID string, manager *pluginruntime.Manager) error {
	if manager == nil {
		return plugins.ErrPluginNotFound
	}
	if c.dispatcher.HasDeliverablePlugin(pluginID) {
		return nil
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	return c.registerRuntime(pluginID, snapshot, manager)
}

func (c *Controller) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *pluginruntime.Manager) error {
	if manager == nil {
		return plugins.ErrPluginNotFound
	}
	concurrency := snapshot.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if max := c.config().Runtime.MaxConcurrentTasksPerPlugin; max > 0 && concurrency > max {
		concurrency = max
	}
	if !c.dispatcher.Register(pluginID, manager, snapshot.Events, snapshot.Commands, concurrency) {
		return dispatch.ErrClosed
	}
	return nil
}

func (c *Controller) dispatchPluginStarted(ctx context.Context, pluginID string) {
	if c.dispatcher == nil {
		return
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return
	}

	now := time.Now()
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, chatevent.Event{
		EventID:        fmt.Sprintf("plugin-started-%s-%d", pluginID, now.UnixNano()),
		SourceProtocol: "platform",
		SourceAdapter:  "plugin.lifecycle",
		EventType:      "plugin.started",
		Timestamp:      now.Unix(),
	})
	if result.Outcome == dispatch.OutcomeDelivered || c.logger == nil {
		return
	}
	pluginLabel := pluginID
	pluginName := ""
	if c.plugins != nil {
		if snapshot, ok := c.plugins.Get(pluginID); ok {
			pluginLabel = plugins.DisplayLabel(snapshot)
			pluginName = snapshot.Name
		}
	}
	c.logger.Warn(
		"插件已启动，但启动通知未送达", "plugin_label", pluginLabel,
		"component", "app",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitTimeoutSeconds
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds+5) * time.Second
}

func runtimeInitBudget(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}
