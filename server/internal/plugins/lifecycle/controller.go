package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
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
	ReleaseRetired(*pluginruntime.Manager)
}

type BotIdentitySource interface {
	BotIdentities() []chatevent.BotIdentity
}

type Settings interface {
	Read(context.Context, string) (map[string]any, error)
	Activate(context.Context, string, map[string]any, func() error) error
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
	Settings            Settings
	Identities          BotIdentitySource
	Webhooks            *pluginwebhook.Registry
	Tasks               *tasks.Registry
	OnRecoveryChange    func(string)
	RefreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	SyncRenderTemplates func(context.Context) error
	Operations          *OperationGate
}

type Controller struct {
	effectiveTimezone   string
	schedulerFailures   logging.FailureTracker
	currentConfig       func() config.Config
	repoRoot            string
	logger              *slog.Logger
	plugins             *plugincatalog.Catalog
	desiredStateRepo    plugins.DesiredStateRepository
	runtimes            RuntimeRegistry
	dispatcher          *dispatch.Dispatcher
	scheduler           *scheduler.Engine
	settings            Settings
	identities          BotIdentitySource
	webhooks            *pluginwebhook.Registry
	tasks               *tasks.Registry
	onRecoveryChange    func(string)
	refreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	syncRenderTemplates func(context.Context) error

	lifecycleCtxMu  sync.RWMutex
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	closed          bool
	workers         sync.WaitGroup
	operations      *OperationGate

	identityMu       sync.Mutex
	identityByPlugin map[string][]chatevent.BotIdentity
}

func NewController(deps Deps) (*Controller, error) {
	if deps.CurrentConfig == nil || deps.Plugins == nil || deps.Runtimes == nil || deps.Dispatcher == nil || deps.Settings == nil {
		return nil, errors.New("plugin lifecycle requires config, catalog, runtimes and dispatcher")
	}
	if deps.Operations == nil {
		return nil, errors.New("plugin lifecycle operation gate is required")
	}
	var zone string
	if deps.Scheduler != nil {
		zone = deps.Scheduler.Timezone()
	} else {
		zone = config.NormalizeTimezone(deps.CurrentConfig().Scheduler.Timezone)
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &Controller{
		lifecycleCtx:        lifecycleCtx,
		lifecycleCancel:     lifecycleCancel,
		effectiveTimezone:   zone,
		currentConfig:       deps.CurrentConfig,
		repoRoot:            deps.RepoRoot,
		logger:              deps.Logger,
		plugins:             deps.Plugins,
		desiredStateRepo:    deps.DesiredStateRepo,
		runtimes:            deps.Runtimes,
		dispatcher:          deps.Dispatcher,
		scheduler:           deps.Scheduler,
		settings:            deps.Settings,
		identities:          deps.Identities,
		webhooks:            deps.Webhooks,
		tasks:               deps.Tasks,
		onRecoveryChange:    deps.OnRecoveryChange,
		refreshManifest:     deps.RefreshManifest,
		syncRenderTemplates: deps.SyncRenderTemplates,
		operations:          deps.Operations,
	}, nil
}

func (c *Controller) BindLifecycleContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.lifecycleCtxMu.Lock()
	previousCancel := c.lifecycleCancel
	c.lifecycleCtx, c.lifecycleCancel = context.WithCancel(ctx)
	if c.closed {
		c.lifecycleCancel()
	}
	c.lifecycleCtxMu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
}

// Close ends admission and waits for accepted asynchronous lifecycle work.
func (c *Controller) Close() {
	c.lifecycleCtxMu.Lock()
	c.closed = true
	c.lifecycleCancel()
	c.lifecycleCtxMu.Unlock()
	c.workers.Wait()
}

func (c *Controller) launch(work func()) bool {
	c.lifecycleCtxMu.Lock()
	defer c.lifecycleCtxMu.Unlock()
	if c.closed || c.lifecycleCtx.Err() != nil {
		return false
	}
	c.workers.Add(1)
	go func() { defer c.workers.Done(); work() }()
	return true
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
	for _, snapshot := range discovered {
		if snapshot.PluginID != pluginID {
			continue
		}
		snapshot.PackageSourceType = current.PackageSourceType
		snapshot.PackageSourceRef = current.PackageSourceRef
		effective := pluginstore.MergeValues(snapshot.DefaultConfig, nil)
		if pluginConfig != nil {
			persisted, err := pluginConfig.ReadAll(ctx, pluginID)
			if err != nil {
				return plugins.Snapshot{}, fmt.Errorf("load persisted plugin settings for %s: %w", pluginID, err)
			}
			effective = pluginstore.MergeValues(snapshot.DefaultConfig, persisted)
		}
		snapshot.Commands = plugincatalog.ProjectCommands(snapshot, effective)
		catalog.RefreshInstalled([]plugins.Snapshot{snapshot}, pluginID)
		updated, ok := catalog.Get(pluginID)
		if !ok {
			return plugins.Snapshot{}, plugins.ErrPluginNotFound
		}
		return updated, nil
	}
	return plugins.Snapshot{}, plugins.ErrPluginNotFound
}

func (c *Controller) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

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
	} else {
		return updated, runtimeErr
	}
	if !c.launch(func() { c.startPluginAsync(updated.PluginID) }) {
		return updated, context.Canceled
	}
	c.reconcileRecoverySummaryBestEffort("plugin.enable")

	return updated, nil
}

func (c *Controller) Disable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

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
			} else {
				return updated, runtimeErr
			}
			if !c.launch(func() { c.stopPluginAsync(pluginID, true) }) {
				return updated, context.Canceled
			}
		default:
			c.dispatcher.Deregister(pluginID)
			c.runtimes.Delete(pluginID)
			manager.ResetCrashCount()
			manager.SetStopped()
			if stoppedSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped)); runtimeErr == nil {
				updated = stoppedSnapshot
			} else {
				return updated, runtimeErr
			}
		}
	}
	c.reconcileRecoverySummaryBestEffort("plugin.disable")

	return updated, nil
}

func (c *Controller) RecoverFromDeadLetter(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

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
		} else {
			return updated, setErr
		}
	}

	manager.ResetCrashCount()
	manager.SetStopped()

	if startingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting)); runtimeErr == nil {
		updated = startingSnapshot
	} else {
		return updated, runtimeErr
	}

	if !c.launch(func() { c.startPluginAsync(updated.PluginID) }) {
		return updated, context.Canceled
	}
	c.reconcileRecoverySummaryBestEffort("plugin.dead_letter_recover")
	return updated, nil
}

func (c *Controller) InvokeManagementAction(ctx context.Context, pluginID, action string, payload map[string]any) (map[string]any, error) {
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
	if err := c.ensurePluginRunning(ctx, pluginID); err != nil {
		return nil, err
	}
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return nil, fmt.Errorf("plugin runtime is not running")
	}

	now := time.Now()
	delivery, err := manager.DeliverEvent(ctx, chatevent.Event{
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

func (c *Controller) acquireOperation(ctx context.Context, pluginID string) (func(), error) {
	c.lifecycleCtxMu.RLock()
	closed := c.closed
	c.lifecycleCtxMu.RUnlock()
	if closed {
		return nil, context.Canceled
	}
	_, release, err := c.operations.Acquire(ctx, pluginID)
	if err == nil {
		c.lifecycleCtxMu.RLock()
		closed = c.closed
		c.lifecycleCtxMu.RUnlock()
		if closed {
			release()
			return nil, context.Canceled
		}
	}
	return release, err
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

func (c *Controller) stopAndResetPlugin(pluginID string) {
	if err := c.stopPlugin(c.lifecycleContext(), pluginID, true); err != nil {
		c.logLifecycleWarn("stop plugin runtime", pluginID, err)
	}
}

// StartInstalled waits for initialization before a package transaction commits.
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

func (c *Controller) StopAndResetPlugin(pluginID string) {
	c.stopAndResetPlugin(pluginID)
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

	ctx, cancel := c.lifecycleTimeoutContext(5 * time.Second)
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

func PluginRuntimeSuperAdmins(cfg config.Config) []string {
	return pluginRuntimeSuperAdmins(cfg)
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
		"插件"+pluginLabel+"已启动，但启动通知未送达。",
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
	if ok && snapshot.DesiredState != "enabled" {
		c.recordSchedulerRunResult(ctx, taskName, job.Revision, scheduler.RunOutcomeOther, time.Since(startedAt), "plugin.event_canceled", "插件已停用，本轮未执行", time.Now())
		return
	}
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, "platform.invalid_request", "plugin is not available")
		return
	}

	if err := c.ensurePluginRunning(ctx, pluginID); err != nil {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, "plugin.internal_error", err.Error())
		return
	}

	pluginName := schedulerPluginDisplayName(snapshot, pluginID)

	result := c.dispatcher.DispatchScheduledEvent(ctx, pluginID, chatevent.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      startedAt.Unix(),
		PayloadFields:  schedulerPayloadFields(job),
	}, scheduler.RunContext{
		JobID:      job.JobID,
		Revision:   job.Revision,
		PluginName: pluginName,
		TaskName:   taskName,
		LogLabel:   logLabel,
		StartedAt:  startedAt,
		Recorder:   c.scheduler,
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.logSchedulerTriggerFailure(ctx, pluginID, pluginName, taskName, logLabel, job.Revision, startedAt, result.ErrorCode, string(result.Outcome))
	} else if count := c.schedulerFailures.Recover(pluginID + ":" + taskName); count > 0 && c.logger != nil {
		c.logger.Info(scheduler.DisplayMessage(pluginName, taskName, logLabel, "已恢复，等待执行"), "component", "scheduler", "plugin_id", pluginID, "job_id", taskName, "repeat_count", count)
	}
}

func (c *Controller) logSchedulerTriggerFailure(ctx context.Context, pluginID, pluginName, taskName, logLabel string, revision uint64, startedAt time.Time, errorCode, errorText string) {
	duration := time.Since(startedAt)
	outcome := scheduler.RunOutcomeFailed
	if errorCode == "plugin.event_canceled" || ctx.Err() == context.Canceled {
		outcome, errorCode, errorText = scheduler.RunOutcomeOther, "plugin.event_canceled", "本轮调度已取消"
	}
	c.recordSchedulerRunResult(ctx, taskName, revision, outcome, duration, errorCode, errorText, time.Now())
	if c.logger == nil {
		return
	}
	if outcome == scheduler.RunOutcomeOther {
		return
	}
	count := c.schedulerFailures.Failure(pluginID+":"+taskName, errorCode, time.Now())
	if count == 0 {
		return
	}
	message := scheduler.DisplayMessage(pluginName, taskName, logLabel, "未执行") + "；插件暂时无法接收任务，请检查插件状态。"
	if count > 1 {
		message += fmt.Sprintf("（期间重复 %d 次）", count)
	}
	c.logger.Warn(
		message,
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"job_id", taskName,
		"log_label", logLabel,
		"duration_ms", duration.Milliseconds(),
		"error_code", errorCode,
		"error", errorText,
		"repeat_count", count,
	)
}

func (c *Controller) recordSchedulerRunResult(ctx context.Context, jobID string, revision uint64, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string, occurredAt time.Time) {
	if c.scheduler == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
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
			"定时任务 "+jobID+" 的结果保存失败，历史记录可能缺失："+err.Error(),
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
	c.reconcileRuntime(ctx)
	c.SyncBotIdentities(ctx)
}

// SyncBotIdentities serializes snapshot capture and admission so concurrent
// adapter callbacks cannot publish an older identity list after a newer one.
func (c *Controller) SyncBotIdentities(ctx context.Context) {
	if c.dispatcher == nil {
		return
	}
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	bots := c.botIdentities()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string][]chatevent.BotIdentity)
	}
	for _, pluginID := range c.dispatcher.PluginIDs() {
		previous, sent := c.identityByPlugin[pluginID]
		if sent && slices.Equal(previous, bots) {
			continue
		}
		now := time.Now()
		event := chatevent.Event{
			EventID:        fmt.Sprintf("bot-identities-%d", now.UnixNano()),
			SourceProtocol: "platform", SourceAdapter: "adapters.internal",
			EventType: "bot.identities.changed", Timestamp: now.Unix(),
			PayloadFields: map[string]any{"bots": append([]chatevent.BotIdentity{}, bots...)},
		}
		result := c.dispatcher.DispatchToPlugin(ctx, pluginID, event)
		if result.Outcome == dispatch.OutcomeDelivered {
			c.identityByPlugin[pluginID] = append([]chatevent.BotIdentity{}, bots...)
		}
	}
}

func (c *Controller) clearBotIdentity(pluginID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	delete(c.identityByPlugin, pluginID)
}

func (c *Controller) botIdentities() []chatevent.BotIdentity {
	if c.identities == nil {
		return []chatevent.BotIdentity{}
	}
	return c.identities.BotIdentities()
}

func (c *Controller) handleCrash(pluginID string, crashCount int, _ string) {
	release, err := c.acquireOperation(c.lifecycleContext(), pluginID)
	if err != nil {
		return
	}
	defer release()
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	current := manager.Snapshot()
	if current.State != pluginruntime.StateCrashed && current.State != pluginruntime.StateStopped {
		return
	}
	if current.CrashCount > 0 {
		crashCount = current.CrashCount
	}
	if c.dispatcher != nil {
		c.dispatcher.Deregister(pluginID)
	}
	c.clearBotIdentity(pluginID)

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	maxRetries := pluginruntime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		manager.SetDeadLetterState()
		runtimeSnapshot := manager.Snapshot()
		c.publishRuntimeState(pluginID, string(pluginruntime.StateDeadLetter))
		if c.plugins != nil && runtimeSnapshot.EnteredDeadLetterAt != nil {
			if _, err := c.plugins.SetDeadLetterSnapshot(pluginID, plugins.DeadLetterSnapshot{
				EnteredAt:        *runtimeSnapshot.EnteredDeadLetterAt,
				CrashCount:       runtimeSnapshot.CrashCount,
				LastErrorCode:    runtimeSnapshot.LastErrorCode,
				LastErrorMessage: runtimeSnapshot.LastErrorMessage,
			}); err != nil {
				c.logLifecycleWarn("publish plugin dead letter state", pluginID, err)
			}
		}
		if c.logger != nil {
			c.logger.Warn(
				fmt.Sprintf("插件%s连续异常退出 %d 次，已停止自动重启，请检查后重新启用。", plugins.DisplayLabel(snapshot), crashCount),
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
	c.publishRuntimeState(pluginID, string(pluginruntime.StateBackoff))

	if c.logger != nil {
		c.logger.Info(
			fmt.Sprintf("插件%s异常退出，%d 秒后第 %d 次重启。", plugins.DisplayLabel(snapshot), int(delay.Seconds()), crashCount),
			"component", "app",
			"plugin_id", pluginID,
			"plugin_name", snapshot.Name,
			"crash_count", crashCount,
			"backoff_seconds", int(delay.Seconds()),
		)
	}

	c.launch(func() { c.backoffRestart(pluginID, delay, manager) })
}

func (c *Controller) HandleCrash(pluginID string, crashCount int, reason string) {
	c.handleCrash(pluginID, crashCount, reason)
}

func (c *Controller) backoffRestart(pluginID string, delay time.Duration, expected *pluginruntime.Manager) {

	lifecycleCtx := c.lifecycleContext()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-lifecycleCtx.Done():
		return
	case <-timer.C:
	}
	release, lockErr := c.acquireOperation(lifecycleCtx, pluginID)
	if lockErr != nil {
		return
	}
	defer release()
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager != expected {
		return
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	if manager.Snapshot().State != pluginruntime.StateBackoff {
		return
	}

	ctx, cancel := context.WithTimeout(lifecycleCtx, runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	c.publishRuntimeState(pluginID, string(pluginruntime.StateStarting))
	if err := c.startRuntimeLocked(ctx, pluginID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		// startRuntime 可能在构建启动输入阶段失败（此时 Manager.Start 尚未执行），
		// manager 会停留在 backoff 状态，之后所有触发都视为等待重试而跳过启动；
		// 重置为 stopped，让下一次 scheduler 触发或管理操作能再次尝试。
		manager.SetStopped()
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
	}
}

func persistPluginDesiredState(ctx context.Context, repo plugins.DesiredStateRepository, pluginID, desiredState string) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
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

func (c *Controller) reconcileRecoverySummaryBestEffort(trigger string) {
	if c.onRecoveryChange == nil {
		return
	}
	c.onRecoveryChange(trigger)
}

func (c *Controller) publishRuntimeState(pluginID, state string) {
	code, message := "", ""
	if state == string(pluginruntime.StateStopped) {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			actual := manager.Snapshot()
			code, message = actual.LastErrorCode, actual.LastErrorMessage
			switch actual.State {
			case pluginruntime.StateStarting, pluginruntime.StateRunning, pluginruntime.StateStopping:
				state = string(actual.State)
			}
		}
	}
	if _, err := c.plugins.SetRuntimeResult(pluginID, state, code, message); err != nil {
		c.logLifecycleWarn("publish plugin runtime state", pluginID, err)
	}
}

func (c *Controller) publishRuntimeFailure(pluginID string, cause error) {
	state := string(pluginruntime.StateStopped)
	if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
		state = string(manager.Snapshot().State)
	}
	code := errorcodes.PluginInternalError
	var runtimeErr *plugins.Error
	if errors.As(cause, &runtimeErr) {
		code = runtimeErr.Code
	}
	definition, ok := errorcodes.Lookup(code)
	message := "插件初始化失败"
	if ok {
		message = definition.Message
	}
	if _, err := c.plugins.SetRuntimeResult(pluginID, state, code, message); err != nil {
		c.logLifecycleWarn("publish plugin initialization failure", pluginID, err)
	}
}

func (c *Controller) logLifecycleWarn(message, pluginID string, err error) {
	if c.logger == nil || err == nil {
		return
	}

	pluginLabel, pluginName := c.pluginLogLabel(pluginID)
	c.logger.Warn(
		"插件"+pluginLabel+lifecycleActionLabel(message)+"失败："+err.Error(),
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
		return "重新加载时启动"
	case "start stopped plugin runtime during reload":
		return "重新加载时启动"
	case "restart plugin runtime during reload":
		return "重新加载时重启"
	case "build runtime spec for plugin reload":
		return "准备启动配置"
	case "reload plugin runtime":
		return "重新加载"
	case "restart plugin after crash backoff":
		return "自动重启"
	case "stop plugin runtime":
		return "停止"
	case "plugin runtime reconcile failed":
		return "启动"
	case "start plugin runtime after enable":
		return "启用后启动"
	case "create plugin reload task":
		return "创建重新加载任务"
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
	taskID, err := c.tasks.Create("plugin.reload", "重新加载插件“"+displayName+"”")
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
		Summary:    lifecycleStringPtr("插件“" + pluginID + "”已重新加载"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件已重新加载",
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func (c *Controller) failReloadTask(taskID string, pluginID string, code string, message string, details ...map[string]any) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if strings.TrimSpace(code) == "" {
		code = errorcodes.PluginInternalError
	}
	if strings.TrimSpace(message) == "" {
		message = "插件重载失败"
	}
	now := time.Now().UTC()
	errorDetails := map[string]any{"plugin_id": pluginID}
	if len(details) > 0 {
		errorDetails = details[0]
	}
	c.tasks.Update(taskID, tasks.Update{
		Status:     lifecycleTaskStatusPtr(tasks.StatusFailed),
		Summary:    lifecycleStringPtr(message),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
			Details: errorDetails,
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
