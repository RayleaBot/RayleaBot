package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type RuntimeRegistry interface {
	Get(pluginID string) (*pluginruntime.Manager, bool)
	GetOrCreate(pluginID string) *pluginruntime.Manager
	NewDetached() *pluginruntime.Manager
	Replace(pluginID string, manager *pluginruntime.Manager) *pluginruntime.Manager
	Delete(pluginID string) *pluginruntime.Manager
}

type BotIdentitySource interface {
	CurrentBotID() string
}

type Deps struct {
	CurrentConfig       func() config.Config
	RepoRoot            string
	Logger              *slog.Logger
	Plugins             *plugincatalog.Catalog
	DesiredStateRepo    plugins.DesiredStateRepository
	Runtimes            RuntimeRegistry
	Dispatcher          *dispatch.Dispatcher
	Scheduler           *scheduler.Engine
	PluginConfig        pluginstore.ConfigRepository
	Adapter             BotIdentitySource
	Webhooks            *pluginwebhook.Registry
	Tasks               *tasks.Registry
	OnRecoveryChange    func(string)
	RefreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	SyncRenderTemplates func(context.Context) error
}

type Controller struct {
	currentConfig       func() config.Config
	repoRoot            string
	logger              *slog.Logger
	plugins             *plugincatalog.Catalog
	desiredStateRepo    plugins.DesiredStateRepository
	runtimes            RuntimeRegistry
	dispatcher          *dispatch.Dispatcher
	scheduler           *scheduler.Engine
	pluginConfig        pluginstore.ConfigRepository
	adapter             BotIdentitySource
	webhooks            *pluginwebhook.Registry
	tasks               *tasks.Registry
	onRecoveryChange    func(string)
	refreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	syncRenderTemplates func(context.Context) error

	lifecycleCtxMu sync.RWMutex
	lifecycleCtx   context.Context

	identityMu       sync.Mutex
	identityByPlugin map[string]string
}

func NewController(deps Deps) *Controller {
	return &Controller{
		currentConfig:       deps.CurrentConfig,
		repoRoot:            deps.RepoRoot,
		logger:              deps.Logger,
		plugins:             deps.Plugins,
		desiredStateRepo:    deps.DesiredStateRepo,
		runtimes:            deps.Runtimes,
		dispatcher:          deps.Dispatcher,
		scheduler:           deps.Scheduler,
		pluginConfig:        deps.PluginConfig,
		adapter:             deps.Adapter,
		webhooks:            deps.Webhooks,
		tasks:               deps.Tasks,
		onRecoveryChange:    deps.OnRecoveryChange,
		refreshManifest:     deps.RefreshManifest,
		syncRenderTemplates: deps.SyncRenderTemplates,
	}
}

func (c *Controller) BindLifecycleContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.lifecycleCtxMu.Lock()
	c.lifecycleCtx = ctx
	c.lifecycleCtxMu.Unlock()
}

func (c *Controller) lifecycleContext() context.Context {
	c.lifecycleCtxMu.RLock()
	ctx := c.lifecycleCtx
	c.lifecycleCtxMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (c *Controller) lifecycleTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.lifecycleContext(), timeout)
}

func (c *Controller) config() config.Config {
	if c.currentConfig == nil {
		return config.Config{}
	}
	return c.currentConfig()
}

type PluginConfigReader interface {
	ReadAll(ctx context.Context, pluginID string) (map[string]any, error)
}

func RefreshPluginManifest(
	ctx context.Context,
	catalog *plugincatalog.Catalog,
	pluginConfig PluginConfigReader,
	pluginID string,
	discover func() ([]plugins.Snapshot, error),
) (plugins.Snapshot, error) {
	if catalog == nil {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	current, ok := catalog.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if discover == nil {
		return current, nil
	}

	discovered, err := discover()
	if err != nil {
		return plugins.Snapshot{}, err
	}
	currentByID := make(map[string]plugins.Snapshot)
	for _, snapshot := range catalog.List() {
		currentByID[snapshot.PluginID] = snapshot
	}
	nextEntries := make([]plugins.Snapshot, 0, len(discovered))
	var refreshed plugins.Snapshot
	found := false
	for _, snapshot := range discovered {
		if existing, ok := currentByID[snapshot.PluginID]; ok {
			snapshot.DesiredState = existing.DesiredState
			snapshot.RuntimeState = existing.RuntimeState
			snapshot.DeadLetter = existing.DeadLetter
			snapshot.PackageSourceType = existing.PackageSourceType
			snapshot.PackageSourceRef = existing.PackageSourceRef
		}
		settings := plugins.CloneSettings(snapshot.DefaultConfig)
		if pluginConfig != nil {
			persisted, err := pluginConfig.ReadAll(ctx, snapshot.PluginID)
			if err != nil {
				return plugins.Snapshot{}, fmt.Errorf("load persisted plugin settings for %s: %w", snapshot.PluginID, err)
			}
			for key, value := range persisted {
				settings[key] = plugins.CloneSettingValue(value)
			}
		}
		snapshot.Commands = plugincatalog.ProjectCommands(snapshot, settings)
		if snapshot.PluginID == pluginID {
			refreshed = snapshot
			found = true
		}
		nextEntries = append(nextEntries, snapshot)
	}
	if !found {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}

	for _, currentSnapshot := range catalog.List() {
		if currentSnapshot.PluginID == pluginID {
			continue
		}
		known := false
		for _, next := range nextEntries {
			if next.PluginID == currentSnapshot.PluginID {
				known = true
				break
			}
		}
		if !known {
			nextEntries = append(nextEntries, currentSnapshot)
		}
	}

	catalog.Replace(nextEntries)
	return refreshed, nil
}

func (c *Controller) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.plugins.SetDesiredState(pluginID, "enabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if runtimeSnapshot, runtimeErr := c.plugins.SetRuntimeState(updated.PluginID, string(pluginruntime.StateStarting)); runtimeErr == nil {
		updated = runtimeSnapshot
	}
	go c.startPluginAsync(updated.PluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.enable")

	return updated, nil
}

func (c *Controller) Disable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c.plugins == nil {
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
		case pluginruntime.StateStarting, pluginruntime.StateRunning, pluginruntime.StateStopping:
			if stoppingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopping)); runtimeErr == nil {
				updated = stoppingSnapshot
			}
			go c.stopPluginAsync(pluginID, true)
		default:
			c.dispatcher.Deregister(pluginID)
			c.runtimes.Delete(pluginID)
			manager.ResetCrashCount()
			manager.SetStopped()
			if stoppedSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped)); runtimeErr == nil {
				updated = stoppedSnapshot
			}
		}
	}
	c.reconcileRecoverySummaryBestEffort("plugin.disable")

	return updated, nil
}

func (c *Controller) RecoverFromDeadLetter(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c.plugins == nil {
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
	if manager.Snapshot().State != pluginruntime.StateDeadLetter {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
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

	if startingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting)); runtimeErr == nil {
		updated = startingSnapshot
	}

	go c.startPluginAsync(updated.PluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.dead_letter_recover")
	return updated, nil
}

func (c *Controller) InvokeManagementAction(ctx context.Context, pluginID, action string, payload map[string]any) (map[string]any, error) {
	if c.plugins == nil || c.runtimes == nil {
		return nil, fmt.Errorf("plugin management action service is not available")
	}
	pluginID = strings.TrimSpace(pluginID)
	action = strings.TrimSpace(action)
	if pluginID == "" || action == "" {
		return nil, fmt.Errorf("plugin management action requires plugin_id and action")
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return nil, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		return nil, fmt.Errorf("plugin is not enabled")
	}
	if err := c.ensurePluginRunning(ctx, pluginID, c.currentBotID()); err != nil {
		return nil, err
	}
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return nil, fmt.Errorf("plugin runtime is not running")
	}

	now := time.Now()
	delivery, err := manager.DeliverEvent(ctx, pluginruntime.Event{
		EventID:        fmt.Sprintf("management-action-%s-%d", action, now.UnixNano()),
		SourceProtocol: "management",
		SourceAdapter:  "management.ui",
		EventType:      "management.action",
		Timestamp:      now.Unix(),
		PayloadFields: map[string]any{
			"action":  action,
			"payload": payload,
		},
	})
	if err != nil {
		return nil, err
	}
	return delivery.Result, nil
}

func (c *Controller) reconcileRuntime(ctx context.Context, botID string) {
	if c.plugins == nil {
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
	if c.runtimes == nil {
		return nil
	}

	manager := c.runtimes.GetOrCreate(pluginID)
	switch manager.Snapshot().State {
	case pluginruntime.StateRunning:
		c.registerRuntimeIfNeeded(pluginID, manager)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateRunning))
		return nil
	case pluginruntime.StateStarting, pluginruntime.StateStopping, pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter:
		return nil
	default:
	}

	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting))
	return c.startRuntime(ctx, pluginID, botID, manager)
}

func (c *Controller) EnsurePluginRunning(ctx context.Context, pluginID, botID string) error {
	return c.ensurePluginRunning(ctx, pluginID, botID)
}

func (c *Controller) startPluginAsync(pluginID, botID string) {
	botID = strings.TrimSpace(botID)

	ctx, cancel := c.lifecycleTimeoutContext(runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	manager := c.runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
	}
}

func (c *Controller) startRuntime(ctx context.Context, pluginID, botID string, manager *pluginruntime.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
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
	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateRunning))
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	return nil
}

func (c *Controller) stopAndResetPlugin(pluginID string) {
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

	ctx, cancel := c.lifecycleTimeoutContext(5 * time.Second)
	defer cancel()
	c.stopPlugin(ctx, pluginID, remove)
}

func (c *Controller) stopPlugin(ctx context.Context, pluginID string, remove bool) {
	if c.runtimes == nil {
		return
	}

	c.clearBotIdentity(pluginID)
	c.dispatcher.Deregister(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	switch manager.Snapshot().State {
	case pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter, pluginruntime.StateStopped:
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
	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
}

func (c *Controller) buildStartInputs(ctx context.Context, pluginID, botID string) (pluginruntime.Spec, pluginruntime.InitPayload, error) {
	_ = ctx
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, plugins.ErrPluginNotFound
	}
	return c.buildStartInputsWithCapabilities(pluginID, botID, c.declaredCapabilities(snapshot))
}

func (c *Controller) declaredCapabilities(snapshot plugins.Snapshot) []string {
	return plugins.DedupeCapabilities(snapshot.DeclaredCapabilities)
}

func (c *Controller) buildStartInputsWithCapabilities(pluginID, botID string, capabilities []string) (pluginruntime.Spec, pluginruntime.InitPayload, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, plugins.ErrPluginNotFound
	}

	cfg := c.config()
	spec, err := pluginruntime.BuildSpec(snapshot, c.repoRoot, cfg.Runtime)
	if err != nil {
		return pluginruntime.Spec{}, pluginruntime.InitPayload{}, err
	}

	payload := pluginruntime.InitPayload{
		Bot: pluginruntime.BotInfo{
			ID: strings.TrimSpace(botID),
		},
		Capabilities:    append([]string(nil), capabilities...),
		SuperAdmins:     pluginRuntimeSuperAdmins(cfg),
		CommandPrefixes: pluginRuntimeCommandPrefixes(cfg),
	}
	return spec, payload, nil
}

func pluginRuntimeCommandPrefixes(cfg config.Config) []string {
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		return sanitizeRuntimeCommandPrefixes(cfg.Command.Prefixes)
	}
	return []string{"/"}
}

func sanitizeRuntimeCommandPrefixes(prefixes []string) []string {
	items := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		items = append(items, prefix)
	}
	if len(items) == 0 {
		return []string{"/"}
	}
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

func PluginRuntimeSuperAdmins(cfg config.Config) []string {
	return pluginRuntimeSuperAdmins(cfg)
}

func (c *Controller) afterRuntimeRegistered(ctx context.Context, pluginID string, initBotID string) {
	c.dispatchPluginStarted(ctx, pluginID)

	initBotID = strings.TrimSpace(initBotID)
	currentBotID := c.currentBotID()
	if initBotID != "" {
		c.markBotIdentitySent(pluginID, initBotID)
		if currentBotID != "" && currentBotID != initBotID {
			c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
		}
		return
	}
	c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
}

func (c *Controller) registerRuntimeIfNeeded(pluginID string, manager *pluginruntime.Manager) {
	if c.dispatcher == nil || manager == nil {
		return
	}
	if c.dispatcher.HasDeliverablePlugin(pluginID) {
		return
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return
	}
	c.registerRuntime(pluginID, snapshot, manager)
}

func (c *Controller) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *pluginruntime.Manager) {
	if c.dispatcher == nil || manager == nil {
		return
	}
	runtimeSnapshot := manager.Snapshot()
	concurrency := snapshot.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if max := c.config().Runtime.MaxConcurrentTasksPerPlugin; max > 0 && concurrency > max {
		concurrency = max
	}
	c.dispatcher.Register(pluginID, manager, runtimeSnapshot.Subscriptions, dispatchCommands(snapshot.Commands), concurrency)
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
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, pluginruntime.Event{
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
		"插件"+pluginLabel+"启动事件投递失败",
		"component", "app",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}

func (c *Controller) HandleSchedulerTrigger(ctx context.Context, job scheduler.Job) {

	pluginID := strings.TrimSpace(job.PluginID)
	if pluginID == "" {
		return
	}
	taskName := strings.TrimSpace(job.JobID)
	logLabel := scheduler.DisplayLabel(job.LogLabel)
	startedAt := time.Now()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, "platform.invalid_request", "plugin is not available")
		return
	}

	if err := c.ensurePluginRunning(ctx, pluginID, c.currentBotID()); err != nil {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, "plugin.internal_error", err.Error())
		return
	}

	pluginName := schedulerPluginDisplayName(snapshot, pluginID)

	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, pluginruntime.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      startedAt.Unix(),
		PayloadFields:  schedulerPayloadFields(job),
		SchedulerLog: &pluginruntime.SchedulerLogContext{
			JobID:      job.JobID,
			Revision:   job.Revision,
			PluginName: pluginName,
			TaskName:   taskName,
			LogLabel:   logLabel,
			StartedAt:  startedAt,
			Recorder:   c.scheduler,
		},
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.logSchedulerTriggerFailure(ctx, pluginID, pluginName, taskName, logLabel, job.Revision, startedAt, result.ErrorCode, string(result.Outcome))
	}
}

func (c *Controller) logSchedulerTriggerFailure(ctx context.Context, pluginID, pluginName, taskName, logLabel string, revision uint64, startedAt time.Time, errorCode, errorText string) {
	duration := time.Since(startedAt)
	c.recordSchedulerRunResult(ctx, taskName, revision, scheduler.RunOutcomeFailed, duration, errorCode, errorText, time.Now())
	if c.logger == nil {
		return
	}
	c.logger.Warn(
		scheduler.DisplayMessage(pluginName, taskName, logLabel, "处理失败")+"耗时 "+scheduler.FormatDuration(duration),
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"job_id", taskName,
		"log_label", logLabel,
		"duration_ms", duration.Milliseconds(),
		"error_code", errorCode,
		"error", errorText,
	)
}

func (c *Controller) recordSchedulerRunResult(ctx context.Context, jobID string, revision uint64, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string, occurredAt time.Time) {
	if c.scheduler == nil {
		return
	}
	if err := c.scheduler.RecordRunResult(ctx, scheduler.RunResult{
		JobID:      jobID,
		Revision:   revision,
		Outcome:    outcome,
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: occurredAt,
	}); err != nil && c.logger != nil {
		c.logger.Warn(
			"定时任务 "+jobID+" 的运行结果保存失败",
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerPluginDisplayName(snapshot plugins.Snapshot, pluginID string) string {
	if name := strings.TrimSpace(snapshot.Name); name != "" {
		return name
	}
	if pluginID = strings.TrimSpace(pluginID); pluginID != "" {
		return pluginID
	}
	return "未知插件"
}

func SchedulerPluginDisplayName(snapshot plugins.Snapshot, pluginID string) string {
	return schedulerPluginDisplayName(snapshot, pluginID)
}

func schedulerPayloadFields(job scheduler.Job) map[string]any {
	fields := make(map[string]any, 2)
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		return fields
	}
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fields
	}
	fields["payload"] = payload
	if action, ok := payload["action"].(string); ok && strings.TrimSpace(action) != "" {
		fields["action"] = action
	}
	return fields
}

func (c *Controller) HandleAdapterReady(ctx context.Context) {
	botID := c.currentBotID()
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *Controller) HandleAdapterBotID(ctx context.Context, botID string) {
	botID = strings.TrimSpace(botID)
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *Controller) broadcastBotIdentityChanged(ctx context.Context, botID string) {
	if c.dispatcher == nil {
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

func (c *Controller) dispatchBotIdentityChangedToPlugin(ctx context.Context, pluginID string, botID string) {
	if c.dispatcher == nil {
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
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, pluginruntime.Event{
		EventID:        fmt.Sprintf("onebot11-bot-identity-%d-%s", now.UnixNano(), botID),
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "bot.identity.changed",
		Timestamp:      now.Unix(),
		Target: &pluginruntime.EventTarget{
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

func (c *Controller) botIdentityAlreadySent(pluginID string, botID string) bool {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	return c.identityByPlugin != nil && c.identityByPlugin[pluginID] == botID
}

func (c *Controller) markBotIdentitySent(pluginID string, botID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string]string)
	}
	c.identityByPlugin[pluginID] = botID
}

func (c *Controller) clearBotIdentity(pluginID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin != nil {
		delete(c.identityByPlugin, pluginID)
	}
}

func (c *Controller) currentBotID() string {
	if c.adapter == nil {
		return ""
	}
	return strings.TrimSpace(c.adapter.CurrentBotID())
}

func (c *Controller) CurrentBotID() string {
	return c.currentBotID()
}

func (c *Controller) handleCrash(pluginID string, crashCount int, _ string) {
	if c.dispatcher != nil {
		c.dispatcher.Deregister(pluginID)
	}
	c.clearBotIdentity(pluginID)

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	maxRetries := pluginruntime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		manager.SetDeadLetterState()
		runtimeSnapshot := manager.Snapshot()
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateDeadLetter))
		if c.plugins != nil && runtimeSnapshot.EnteredDeadLetterAt != nil {
			_, _ = c.plugins.SetDeadLetterSnapshot(pluginID, plugins.DeadLetterSnapshot{
				EnteredAt:        *runtimeSnapshot.EnteredDeadLetterAt,
				CrashCount:       runtimeSnapshot.CrashCount,
				LastErrorCode:    runtimeSnapshot.LastErrorCode,
				LastErrorMessage: runtimeSnapshot.LastErrorMessage,
			})
		}
		if c.webhooks != nil {
			c.webhooks.DeletePlugin(pluginID)
		}
		if c.logger != nil {
			c.logger.Warn(
				"插件"+plugins.DisplayLabel(snapshot)+"连续崩溃，已进入死信状态",
				"component", "app",
				"plugin_id", pluginID,
				"plugin_name", snapshot.Name,
				"crash_count", crashCount,
				"max_retries", maxRetries,
			)
		}
		return
	}

	cfg := c.config().Runtime
	delay := pluginruntime.CrashBackoff(crashCount, cfg.CrashBackoffInitialSeconds, cfg.CrashBackoffMaxSeconds)
	nextRetry := time.Now().Add(delay)

	manager.SetBackoffState(nextRetry)
	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateBackoff))

	if c.logger != nil {
		c.logger.Info(
			"插件"+plugins.DisplayLabel(snapshot)+"运行时崩溃，等待重启",
			"component", "app",
			"plugin_id", pluginID,
			"plugin_name", snapshot.Name,
			"crash_count", crashCount,
			"backoff_seconds", int(delay.Seconds()),
		)
	}

	go c.backoffRestart(pluginID, delay)
}

func (c *Controller) HandleCrash(pluginID string, crashCount int, reason string) {
	c.handleCrash(pluginID, crashCount, reason)
}

func (c *Controller) backoffRestart(pluginID string, delay time.Duration) {

	lifecycleCtx := c.lifecycleContext()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-lifecycleCtx.Done():
		return
	case <-timer.C:
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	if manager.Snapshot().State != pluginruntime.StateBackoff {
		return
	}

	botID := c.currentBotID()

	ctx, cancel := context.WithTimeout(lifecycleCtx, runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting))
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
	}
}

func persistPluginDesiredState(ctx context.Context, repo plugins.DesiredStateRepository, pluginID, desiredState string) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
}

func dispatchCommands(commands []plugins.Command) []dispatch.CommandDecl {
	items := make([]dispatch.CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, dispatch.CommandDecl{
			Name:         command.Name,
			Aliases:      append([]string(nil), command.Aliases...),
			MatchPattern: command.MatchPattern,
			Permission:   command.Permission,
		})
	}
	return items
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}

func (c *Controller) seedPluginDefaultConfig(ctx context.Context, snapshot plugins.Snapshot) error {
	if c.pluginConfig == nil || len(snapshot.DefaultConfig) == 0 {
		return nil
	}
	_, err := c.pluginConfig.SeedDefaults(ctx, snapshot.PluginID, snapshot.DefaultConfig)
	return err
}

func (c *Controller) reconcileRecoverySummaryBestEffort(trigger string) {
	if c.onRecoveryChange == nil {
		return
	}
	c.onRecoveryChange(trigger)
}

func (c *Controller) logLifecycleWarn(message, pluginID string, err error) {
	if c.logger == nil || err == nil {
		return
	}

	pluginLabel, pluginName := c.pluginLogLabel(pluginID)
	c.logger.Warn(
		"插件"+pluginLabel+lifecycleActionLabel(message)+"失败",
		"component", "app",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"err", err.Error(),
	)
}

func (c *Controller) pluginLogLabel(pluginID string) (string, string) {
	pluginID = strings.TrimSpace(pluginID)
	if c != nil && c.plugins != nil {
		if snapshot, ok := c.plugins.Get(pluginID); ok {
			return plugins.DisplayLabel(snapshot), snapshot.Name
		}
	}
	if pluginID == "" {
		return "未知插件", ""
	}
	return pluginID, ""
}

func lifecycleActionLabel(message string) string {
	switch strings.TrimSpace(message) {
	case "start plugin runtime during reload":
		return "重载时启动运行时"
	case "start stopped plugin runtime during reload":
		return "重载时启动已停止的运行时"
	case "restart plugin runtime during reload":
		return "重载时重启运行时"
	case "build runtime spec for plugin reload":
		return "重载时生成运行时配置"
	case "reload plugin runtime":
		return "重载运行时"
	case "restart plugin after crash backoff":
		return "崩溃等待后重启"
	case "stop plugin runtime":
		return "停止运行时"
	case "plugin runtime reconcile failed":
		return "启动"
	case "start plugin runtime after enable":
		return "启用后启动运行时"
	case "create plugin reload task":
		return "创建重载任务"
	default:
		if strings.TrimSpace(message) == "" {
			return "处理"
		}
		return "处理：" + strings.TrimSpace(message)
	}
}

func (c *Controller) createReloadTask(pluginID string, snapshot plugins.Snapshot) string {
	if c.tasks == nil {
		return ""
	}
	displayName := strings.TrimSpace(snapshot.Name)
	if displayName == "" {
		displayName = pluginID
	}
	taskID, err := c.tasks.Create("plugin.reload", "reload plugin: "+displayName)
	if err != nil {
		c.logLifecycleWarn("create plugin reload task", pluginID, err)
		return ""
	}
	return taskID
}

func (c *Controller) startReloadTask(taskID string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:    lifecycleTaskStatusPtr(tasks.StatusRunning),
		Progress:  lifecycleIntPtr(5),
		Summary:   lifecycleStringPtr("准备重载插件"),
		StartedAt: &now,
	})
}

func (c *Controller) updateReloadTask(taskID string, progress int, summary string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	c.tasks.Update(taskID, tasks.Update{
		Progress: lifecycleIntPtr(progress),
		Summary:  lifecycleStringPtr(summary),
	})
}

func (c *Controller) finishReloadTask(taskID string, pluginID string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:     lifecycleTaskStatusPtr(tasks.StatusSucceeded),
		Progress:   lifecycleIntPtr(100),
		Summary:    lifecycleStringPtr("插件重载完成"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件运行时已重载",
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func (c *Controller) failReloadTask(taskID string, pluginID string, code string, message string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if strings.TrimSpace(code) == "" {
		code = "plugin.internal_error"
	}
	if strings.TrimSpace(message) == "" {
		message = "插件重载失败"
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:     lifecycleTaskStatusPtr(tasks.StatusFailed),
		Summary:    lifecycleStringPtr(message),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func lifecycleStringPtr(value string) *string {
	return &value
}

func lifecycleIntPtr(value int) *int {
	return &value
}

func lifecycleTaskStatusPtr(status tasks.Status) *tasks.Status {
	return &status
}
