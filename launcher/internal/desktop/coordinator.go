package desktop

import (
	"context"
	"errors"
	"sync"
	"time"
)

const repositoryURL = "https://github.com/RayleaBot/RayleaBot"

var errStartupBlocked = errors.New("启动操作已被停止或退出流程阻止")

type DesktopHost interface {
	Emit(name string, data any)
	SetTrayState(state TrayMenuState)
	OpenURL(value string) error
	OpenDirectory(path string) error
	ConfirmExternalServiceStop() bool
}

type operationContext struct {
	settings         LauncherSettings
	resolvedSettings LauncherResolvedSettings
	endpoint         ServerEndpoint
	endpointWarning  string
}

type snapshotOptions struct {
	health               any
	readiness            any
	systemStatus         any
	processLifecycle     string
	processOwnership     string
	lastLocalError       string
	statusHint           string
	localRecoverySummary any
	runtimePrepare       *RuntimePrepareSnapshot
}

type Coordinator struct {
	settingsStore *SettingsStore
	process       *ProcessController
	management    *ManagementClient
	release       *ReleaseFeed
	host          DesktopHost
	watcherPID    int

	mu          sync.RWMutex
	settings    LauncherSettings
	initialized bool
	snapshot    LauncherSnapshot

	initMu       sync.Mutex
	operationMu  sync.Mutex
	startups     startupGate
	releaseMu    sync.Mutex
	publishMu    sync.Mutex
	monitorOnce  sync.Once
	monitorStop  context.CancelFunc
	trayState    TrayMenuState
	trayStateSet bool
}

func NewCoordinator(basePath string, initialControlToken string, watcherPID int, host DesktopHost) *Coordinator {
	process := NewProcessController(initialControlToken)
	coordinator := &Coordinator{
		settingsStore: NewSettingsStore(basePath),
		process:       process,
		host:          host,
		watcherPID:    watcherPID,
	}
	coordinator.management = NewManagementClient(process.ControlToken)
	coordinator.release = NewReleaseFeed(basePath)
	coordinator.snapshot = defaultSnapshot()
	return coordinator
}

func (c *Coordinator) Initialize() error {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return c.Refresh()
	}
	c.mu.Unlock()
	settings, err := c.settingsStore.Load()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = settings
	c.initialized = true
	c.mu.Unlock()
	c.process.SetWorkdir(ResolveLauncherSettings(settings).Workdir)
	if err := c.Refresh(); err != nil {
		return err
	}
	c.monitorOnce.Do(func() {
		monitorContext, cancel := context.WithCancel(context.Background())
		c.monitorStop = cancel
		go c.monitor(monitorContext)
		go c.refreshRelease(false)
	})
	return nil
}

func (c *Coordinator) Snapshot() LauncherSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneSnapshot(c.snapshot)
}

func (c *Coordinator) Shutdown() {
	c.startups.block(true)
	c.initMu.Lock()
	if stop := c.monitorStop; stop != nil {
		stop()
		c.monitorStop = nil
	}
	c.initMu.Unlock()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if !c.process.IsRunning() {
		return
	}
	operation, err := c.operationContext()
	if err == nil && c.quickHealthy(operation.endpoint) {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		_ = c.management.Shutdown(ctx, operation.endpoint)
		cancel()
	}
	deadline := time.Now().Add(4 * time.Second)
	for c.process.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if c.process.IsRunning() {
		_ = c.process.ForceKill()
	}
}

func (c *Coordinator) operationContext() (operationContext, error) {
	c.mu.RLock()
	settings := c.settings
	initialized := c.initialized
	c.mu.RUnlock()
	if !initialized {
		return operationContext{}, errors.New("启动器尚未初始化")
	}
	resolved := ResolveLauncherSettings(settings)
	endpoint, endpointWarning := ResolveServerEndpoint(resolved.ConfigPath)
	return operationContext{settings: settings, resolvedSettings: resolved, endpoint: endpoint, endpointWarning: endpointWarning}, nil
}
