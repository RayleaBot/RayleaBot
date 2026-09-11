package desktop

import (
	"context"
	"errors"
	"sync"
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
	health               *ServerLivenessStatusResponse
	readiness            *ServerReadinessStatusResponse
	systemStatus         *ServerSystemStatusResponse
	processLifecycle     LauncherProcessLifecycle
	processOwnership     LauncherProcessOwnership
	lastLocalError       string
	statusHint           string
	localRecoverySummary *ServerRecoveryCompatibilitySummary
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

func (c *Coordinator) Shutdown() error {
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
		return nil
	}
	operation, operationErr := c.operationContext()
	return stopManagedProcess(c.process, func() error {
		if operationErr != nil {
			return operationErr
		}
		if !c.quickHealthy(operation.endpoint) {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		return c.management.Shutdown(ctx, operation.endpoint)
	}, shutdownGracePeriod)

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
