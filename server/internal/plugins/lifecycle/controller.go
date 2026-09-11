package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
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
	ShutdownTimeout     time.Duration
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
	shutdownTimeout     time.Duration

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
	if deps.ShutdownTimeout <= 0 {
		deps.ShutdownTimeout = 5 * time.Second
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
		shutdownTimeout:     deps.ShutdownTimeout,
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

// Close ends admission and waits for all accepted asynchronous lifecycle work.
// Runtime process shutdown remains owned by the runtime registry.
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

func (c *Controller) changeDesiredState(ctx context.Context, pluginID, desired string) (plugins.Snapshot, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == desired {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}
	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, desired); err != nil {
		return plugins.Snapshot{}, err
	}
	return c.plugins.SetDesiredState(pluginID, desired)
}

func (c *Controller) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

	updated, err := c.changeDesiredState(ctx, pluginID, "enabled")
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

	updated, err := c.changeDesiredState(ctx, pluginID, "disabled")
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

func persistPluginDesiredState(ctx context.Context, repo plugins.DesiredStateRepository, pluginID, desiredState string) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
}
