package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
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

	initMu          sync.Mutex
	operationMu     sync.Mutex
	startupMu       sync.Mutex
	startupNextID   uint64
	startupCancels  map[uint64]context.CancelFunc
	startupBlockers int
	shutdownStarted bool
	releaseMu       sync.Mutex
	publishMu       sync.Mutex
	monitorOnce     sync.Once
	monitorStop     context.CancelFunc
	trayState       TrayMenuState
	trayStateSet    bool
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

func (c *Coordinator) Refresh() error {
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	return c.refreshCurrent()
}

func (c *Coordinator) refreshCurrent() error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	return c.refresh(operation)
}

func (c *Coordinator) refreshCurrentPassive() error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	previouslyWritable := false
	for _, check := range c.Snapshot().Launcher.PreflightChecks {
		if check.Code == "workdir.ready" && check.Severity == "ok" {
			previouslyWritable = true
			break
		}
	}
	return c.refreshWithInspection(operation, inspectEnvironmentPassive(operation.resolvedSettings, previouslyWritable))
}

func (c *Coordinator) refresh(operation operationContext) error {
	return c.refreshWithInspection(operation, InspectEnvironment(operation.resolvedSettings))
}

func (c *Coordinator) refreshWithInspection(operation operationContext, inspection EnvironmentInspection) error {
	recovery := readRecoverySummary(c.process.LogDirectory())
	if inspection.HasBlockingIssues || inspection.CanBootstrapUserConfig {
		lifecycle := "stopped"
		ownership := "none"
		if c.process.IsRunning() {
			lifecycle = "running"
			ownership = "launcher_managed"
		}
		hint := "服务尚未启动。"
		if inspection.CanBootstrapUserConfig {
			hint = "启动服务时会先生成 config/user.yaml。"
		} else if issue := primaryEnvironmentIssue(inspection.PreflightChecks); issue != nil {
			hint = issue.Summary + " " + issue.Remediation
		}
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			processLifecycle: lifecycle, processOwnership: ownership, statusHint: strings.TrimSpace(hint), localRecoverySummary: recovery,
		}))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	healthy := c.management.IsHealthy(ctx, operation.endpoint)
	if !healthy {
		lifecycle := "stopped"
		ownership := "none"
		hint := "服务尚未启动。"
		lastError := ""
		if c.process.IsRunning() {
			lifecycle, ownership = "running", "launcher_managed"
			hint, lastError = "服务进程仍在运行，但健康检查失败。", "健康检查失败。"
		} else if c.developmentWatcherActive() {
			lifecycle, ownership = "starting", "external"
			hint = fmt.Sprintf("开发 watcher 正在重启服务（PID %d），启动器不会重复启动。", c.watcherPID)
		}
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			processLifecycle: lifecycle, processOwnership: ownership, statusHint: hint, lastLocalError: lastError, localRecoverySummary: recovery,
		}))
		return nil
	}

	readiness, err := c.management.GetReadiness(ctx, operation.endpoint)
	if err != nil {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			health: JSONObject{"status": "ok"}, processLifecycle: lifecycleFor(c.process.IsRunning()), processOwnership: ownershipFor(c.process.IsRunning(), true),
			statusHint: "服务存活，但无法读取正式就绪状态。", lastLocalError: err.Error(), localRecoverySummary: recovery,
		}))
		return nil
	}
	var systemStatus any
	if status := objectStatus(readiness); status == "ready" || status == "degraded" {
		if value, statusErr := c.management.GetLauncherStatus(ctx, operation.endpoint); statusErr == nil {
			systemStatus = value
		}
	}
	recovery = recoveryFromPayload(systemStatus, readiness, recovery)
	lifecycle := lifecycleFor(c.process.IsRunning())
	if statusObject, ok := systemStatus.(JSONObject); ok && objectStatus(statusObject) == "shutting_down" {
		lifecycle = "stopping"
	}
	c.process.ClearRuntimePrepare()
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		health: JSONObject{"status": "ok"}, readiness: readiness, systemStatus: systemStatus,
		processLifecycle: lifecycle, processOwnership: ownershipFor(c.process.IsRunning(), true), localRecoverySummary: recovery,
	}))
	return nil
}

func (c *Coordinator) Start() error {
	startupContext, finishStartup, allowed := c.beginStartup()
	if !allowed {
		return errStartupBlocked
	}
	defer finishStartup()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	return c.startLocked(startupContext)
}

func (c *Coordinator) startLocked(startupContext context.Context) error {
	if startupContext == nil {
		startupContext = context.Background()
	}
	if startupContext.Err() != nil {
		return nil
	}
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	inspection := InspectEnvironment(operation.resolvedSettings)
	if inspection.CanBootstrapUserConfig {
		if err := c.process.RunOffline(operation.resolvedSettings, "config", "init"); err != nil {
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "无法生成用户配置。", lastLocalError: err.Error()}))
			return nil
		}
		operation, _ = c.operationContext()
		inspection = InspectEnvironment(operation.resolvedSettings)
		if inspection.CanBootstrapUserConfig {
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "无法生成用户配置。", lastLocalError: "配置初始化命令已完成，但 config/user.yaml 仍未生成。"}))
			return nil
		}
	}
	if inspection.HasBlockingIssues {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: environmentIssueDetail(inspection)}))
		return nil
	}
	ctx, cancel := context.WithTimeout(startupContext, 6*time.Second)
	healthy := c.management.IsHealthy(ctx, operation.endpoint)
	cancel()
	if startupContext.Err() != nil {
		return nil
	}
	if healthy {
		return c.refresh(operation)
	}
	if c.developmentWatcherActive() && !c.process.IsRunning() {
		c.process.WriteLauncherLog(fmt.Sprintf("忽略重复启动请求：开发 watcher PID %d 正在管理 %s。", c.watcherPID, operation.endpoint.BaseURL), operation.resolvedSettings.Workdir)
		return c.refresh(operation)
	}
	if endpointListening(operation.endpoint) && !c.process.IsRunning() {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			processLifecycle: "stopped", processOwnership: "none", statusHint: "目标端口已被现有进程占用，启动器不会重复拉起服务。", lastLocalError: fmt.Sprintf("端口 %d 已被占用。", operation.endpoint.Port),
		}))
		return nil
	}
	if startupContext.Err() != nil {
		return nil
	}
	if err := c.process.Start(operation.resolvedSettings); err != nil {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "无法启动服务进程。", lastLocalError: err.Error()}))
		return nil
	}
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		processLifecycle: "starting", processOwnership: "launcher_managed", statusHint: "正在准备运行环境并等待服务就绪。", runtimePrepare: c.process.RuntimePrepare(),
	}))

	deadline := time.Now().Add(15 * time.Minute)
	failedSince := time.Time{}
	var lastFailed JSONObject
	for time.Now().Before(deadline) {
		if startupContext.Err() != nil {
			return nil
		}
		if !c.process.IsRunning() {
			if c.quickHealthy(operation.endpoint) {
				return c.refresh(operation)
			}
			lastError := "服务进程在通过健康检查前退出。"
			if diagnostics := c.process.RecentStderr(); len(diagnostics) > 0 {
				lastError = diagnostics[len(diagnostics)-1]
			}
			if endpointListening(operation.endpoint) {
				c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "目标端口已被现有进程占用，RayleaBot 无法监听当前地址。", lastLocalError: lastError}))
				return nil
			}
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "服务进程在启动阶段提前退出。", lastLocalError: lastError}))
			return nil
		}
		ctx, cancel := context.WithTimeout(startupContext, 3*time.Second)
		if c.management.IsHealthy(ctx, operation.endpoint) {
			if readiness, readErr := c.management.GetReadiness(ctx, operation.endpoint); readErr == nil {
				if objectStatus(readiness) == "failed" {
					lastFailed = readiness
					if failedSince.IsZero() {
						failedSince = time.Now()
					}
					if time.Since(failedSince) < 10*time.Second {
						cancel()
						if !waitForStartup(startupContext, 500*time.Millisecond) {
							return nil
						}
						continue
					}
				}
				cancel()
				return c.refresh(operation)
			}
		}
		cancel()
		if startupContext.Err() != nil {
			return nil
		}
		if progress := c.process.RuntimePrepare(); progress != nil {
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
				processLifecycle: "starting", processOwnership: "launcher_managed", statusHint: firstNonEmpty(progress.Summary, "正在准备运行环境并等待服务就绪。"), runtimePrepare: progress,
			}))
		}
		if !waitForStartup(startupContext, 500*time.Millisecond) {
			return nil
		}
	}
	if startupContext.Err() != nil {
		return nil
	}
	if lastFailed != nil {
		return c.refresh(operation)
	}
	killErr := c.process.ForceKill()
	return c.finishStartupTimeout(operation, inspection, killErr, c.process.IsRunning())
}

func (c *Coordinator) finishStartupTimeout(operation operationContext, inspection EnvironmentInspection, killErr error, running bool) error {
	options := snapshotOptions{
		processLifecycle: "stopped",
		processOwnership: "none",
		statusHint:       "启动超时内未通过健康检查。",
		lastLocalError:   "服务启动已超时。",
	}
	if running {
		options.processLifecycle = "running"
		options.processOwnership = "launcher_managed"
		options.statusHint = "服务启动已超时，进程仍在运行。"
		options.lastLocalError = "无法确认服务进程已停止。"
	}
	if killErr != nil {
		options.statusHint = "服务启动已超时，且无法终止服务进程。"
		options.lastLocalError = "服务进程终止失败。"
	}
	c.publish(c.buildSnapshot(operation, inspection, options))
	if killErr != nil {
		return fmt.Errorf("服务启动超时且无法终止服务进程：%w", killErr)
	}
	return nil
}

func (c *Coordinator) Stop() error {
	unblockStartups := c.blockStartups(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	return c.stopLocked(true)
}

func (c *Coordinator) stopLocked(confirmExternal bool) error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	inspection := InspectEnvironment(operation.resolvedSettings)
	healthy := c.quickHealthy(operation.endpoint)
	managed := c.process.IsRunning()
	ownership := "none"
	if managed {
		ownership = "launcher_managed"
	} else if healthy {
		ownership = "external"
	}
	if ownership == "external" && confirmExternal {
		if c.host == nil || !c.host.ConfirmExternalServiceStop() {
			return c.refresh(operation)
		}
		if !isLoopbackHost(operation.endpoint.Host) {
			snapshot := c.Snapshot()
			snapshot.Launcher.StatusHint = "无法停止非本机服务。"
			snapshot.Launcher.LastLocalError = "远程服务只能通过管理界面操作。"
			c.publish(snapshot)
			return nil
		}
	}
	if ownership == "external" {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			health: JSONObject{"status": "ok"}, processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止现有服务。",
		}))
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		shutdownErr := c.management.Shutdown(ctx, operation.endpoint)
		cancel()
		if shutdownErr == nil {
			return c.refresh(operation)
		}
		_ = c.refresh(operation)
		snapshot := c.Snapshot()
		snapshot.Launcher.StatusHint = "无法停止检测到的现有服务。"
		snapshot.Launcher.LastLocalError = shutdownErr.Error()
		c.publish(snapshot)
		return nil
	}
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		health: func() any {
			if healthy {
				return JSONObject{"status": "ok"}
			}
			return nil
		}(),
		processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止服务。",
	}))
	if healthy {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		err = c.management.Shutdown(ctx, operation.endpoint)
		cancel()
	}
	if managed {
		deadline := time.Now().Add(5 * time.Second)
		for c.process.IsRunning() && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if c.process.IsRunning() {
			if killErr := c.process.ForceKill(); killErr != nil {
				return killErr
			}
		}
	}
	return c.refresh(operation)
}

func (c *Coordinator) ResetAdmin() error {
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	inspection := InspectEnvironment(operation.resolvedSettings)
	healthy := c.quickHealthy(operation.endpoint)
	managed := c.process.IsRunning()
	if managed || healthy {
		ownership := "external"
		if managed {
			ownership = "launcher_managed"
		}
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止服务以执行管理员重置。"}))
		if managed {
			if err := c.process.ForceKill(); err != nil {
				return err
			}
		} else {
			if !isLoopbackHost(operation.endpoint.Host) {
				return errors.New("无法重置非本机服务的管理员凭据")
			}
			stopped, stopErr := stopEndpointProcess(operation.endpoint)
			if stopErr != nil {
				return stopErr
			}
			if !stopped {
				return errors.New("无法确认现有服务进程，管理员凭据未重置")
			}
		}
	}
	if err := c.process.RunOffline(operation.resolvedSettings, "reset-admin"); err != nil {
		return err
	}
	startupContext, finishStartup, allowed := c.beginStartup()
	if !allowed {
		return fmt.Errorf("管理员凭据已重置，但服务重启被阻止: %w", errStartupBlocked)
	}
	defer finishStartup()
	if err := c.startLocked(startupContext); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if startupContext.Err() != nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(startupContext, 3*time.Second)
		readiness, readErr := c.management.GetReadiness(ctx, operation.endpoint)
		cancel()
		if readErr == nil && objectStatus(readiness) == "setup_required" {
			return c.OpenWebUI("")
		}
		if !waitForStartup(startupContext, 500*time.Millisecond) {
			return nil
		}
	}
	return errors.New("管理员凭据已重置，但服务未在预期时间内进入 setup_required 状态")
}

func (c *Coordinator) SaveSettings(settings LauncherSettings) error {
	normalized, err := normalizeSettings(settings, "")
	if err != nil {
		return err
	}
	unblockStartups, cancelledStartup := c.blockStartupsWithState(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if cancelledStartup && c.Snapshot().Launcher.ProcessLifecycle == "starting" && c.process.IsRunning() {
		if err := c.process.ForceKill(); err != nil {
			return fmt.Errorf("无法停止正在使用旧设置启动的服务：%w", err)
		}
	}
	if err := c.settingsStore.Save(normalized); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = normalized
	c.mu.Unlock()
	if !c.process.IsRunning() {
		c.process.SetWorkdir(ResolveLauncherSettings(normalized).Workdir)
	}
	return c.refreshCurrent()
}

func (c *Coordinator) PreviewResolvedSettings(settings LauncherSettings) (LauncherResolvedSettings, error) {
	normalized, err := normalizeSettings(settings, "")
	if err != nil {
		return LauncherResolvedSettings{}, err
	}
	return ResolveLauncherSettings(normalized), nil
}

func (c *Coordinator) OpenWebUI(targetPath string) error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	normalized, err := sanitizeWebTargetPath(targetPath)
	if err != nil {
		return err
	}
	baseURL := operation.endpoint.BaseURL
	if override := strings.TrimSpace(os.Getenv("RAYLEA_WEB_UI_BASE_URL")); override != "" {
		if candidate, parseErr := url.Parse(override); parseErr == nil && (candidate.Scheme == "http" || candidate.Scheme == "https") && candidate.User == nil {
			baseURL = candidate.String()
		}
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if normalized != "" {
		base, err = base.Parse(normalized)
		if err != nil {
			return err
		}
	} else if snapshot := c.Snapshot(); snapshot.Server.Readiness != nil && readinessStatus(snapshot.Server.Readiness) == "setup_required" {
		if token := c.process.SetupToken(); token != "" {
			base.Path = "/setup"
			base.Fragment = "setup_token=" + token
		}
	}
	return c.host.OpenURL(base.String())
}

func (c *Coordinator) OpenReleasePage() error {
	snapshot := c.Snapshot()
	page := snapshot.Launcher.ReleaseCheck.ReleasePageURL
	if page == "" {
		snapshot.Launcher.StatusHint = "没有可打开的版本页面。"
		c.publish(snapshot)
		return nil
	}
	return c.host.OpenURL(page)
}

func (c *Coordinator) OpenRepositoryPage() error {
	return c.host.OpenURL(repositoryURL)
}

func (c *Coordinator) OpenLogsDirectory() error {
	directory := c.process.LogDirectory()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return c.host.OpenDirectory(directory)
}

func (c *Coordinator) CheckForUpdates() { go c.refreshRelease(true) }

func (c *Coordinator) DownloadUpdate() { go c.downloadRelease() }

func (c *Coordinator) InstallDownloadedUpdate() error {
	// Acquire the release lane before the operation lane so a queued install
	// cannot block stop or shutdown while a check or download is still active.
	c.releaseMu.Lock()
	defer c.releaseMu.Unlock()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	launcherSnapshot := c.Snapshot().Launcher
	releaseCheck := launcherSnapshot.ReleaseCheck
	if releaseCheck.Status == "installing" {
		return nil
	}
	if !releaseReadyForInstall(releaseCheck) {
		return errors.New("没有已验证且可安装的更新")
	}
	if launcherSnapshot.ProcessOwnership == "external" {
		return errors.New("检测到外部服务正在运行，请先停止服务后再安装更新")
	}
	serviceWasRunning := c.process.IsRunning()
	if serviceWasRunning {
		if err := c.stopLocked(true); err != nil {
			return err
		}
	}
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	result, err := c.release.Install(os.Getpid(), serviceWasRunning, operation.resolvedSettings)
	c.publishRelease(result)
	if err != nil && serviceWasRunning {
		return c.recoverServiceAfterUpdateFailure(err)
	}
	return err
}

func releaseReadyForInstall(snapshot ReleaseCheckSnapshot) bool {
	return snapshot.CanInstall && (snapshot.Status == "ready_to_install" || snapshot.Status == "failed")
}

func (c *Coordinator) recoverServiceAfterUpdateFailure(installErr error) error {
	startupContext, finishStartup, allowed := c.beginStartup()
	if !allowed {
		return fmt.Errorf("更新助手启动失败，且原服务恢复失败：恢复启动被阻止；更新错误：%w", installErr)
	}
	defer finishStartup()

	recoveryErr := c.startLocked(startupContext)
	if recoveryErr == nil {
		if !serviceAvailable(c.Snapshot()) {
			recoveryErr = errors.New("服务未恢复到可用状态")
		}
	}
	if recoveryErr != nil {
		return fmt.Errorf("更新助手启动失败，且原服务恢复失败：%v；更新错误：%w", recoveryErr, installErr)
	}
	return installErr
}

func (c *Coordinator) Shutdown() {
	c.blockStartups(true)
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

func (c *Coordinator) beginStartup() (context.Context, func(), bool) {
	c.startupMu.Lock()
	if c.shutdownStarted || c.startupBlockers > 0 {
		c.startupMu.Unlock()
		return nil, nil, false
	}
	startupContext, cancel := context.WithCancel(context.Background())
	c.startupNextID++
	startupID := c.startupNextID
	if c.startupCancels == nil {
		c.startupCancels = make(map[uint64]context.CancelFunc)
	}
	c.startupCancels[startupID] = cancel
	c.startupMu.Unlock()
	var finishOnce sync.Once
	return startupContext, func() {
		finishOnce.Do(func() {
			c.startupMu.Lock()
			delete(c.startupCancels, startupID)
			c.startupMu.Unlock()
			cancel()
		})
	}, true
}

func (c *Coordinator) blockStartups(permanent bool) func() {
	unblock, _ := c.blockStartupsWithState(permanent)
	return unblock
}

func (c *Coordinator) blockStartupsWithState(permanent bool) (func(), bool) {
	c.startupMu.Lock()
	if permanent {
		c.shutdownStarted = true
	} else {
		c.startupBlockers++
	}
	cancels := make([]context.CancelFunc, 0, len(c.startupCancels))
	for _, cancel := range c.startupCancels {
		cancels = append(cancels, cancel)
	}
	hadActiveStartup := len(cancels) > 0
	c.startupMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if permanent {
		return func() {}, hadActiveStartup
	}
	var unblockOnce sync.Once
	return func() {
		unblockOnce.Do(func() {
			c.startupMu.Lock()
			if c.startupBlockers > 0 {
				c.startupBlockers--
			}
			c.startupMu.Unlock()
		})
	}, hadActiveStartup
}

func waitForStartup(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
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

func (c *Coordinator) buildSnapshot(operation operationContext, inspection EnvironmentInspection, options snapshotOptions) LauncherSnapshot {
	if operation.endpointWarning != "" {
		warning := EnvironmentCheckResult{
			Scope: "advisory", Code: "config.endpoint_fallback", Title: "服务监听配置", Severity: "warning",
			Summary: operation.endpointWarning, Detail: operation.endpointWarning,
			Remediation: "请检查 config/user.yaml 中的 server.host 与 server.port。",
		}
		inspection.Checks = append(append([]EnvironmentCheckResult(nil), inspection.Checks...), warning)
		inspection.AdvisoryChecks = append(append([]EnvironmentCheckResult(nil), inspection.AdvisoryChecks...), warning)
	}
	c.mu.RLock()
	currentRelease := cloneReleaseCheck(c.snapshot.Launcher.ReleaseCheck)
	currentOwnership := c.snapshot.Launcher.ProcessOwnership
	c.mu.RUnlock()
	lifecycle := options.processLifecycle
	if lifecycle == "" {
		lifecycle = lifecycleFor(c.process.IsRunning())
	}
	ownership := options.processOwnership
	if ownership == "" {
		ownership = currentOwnership
	}
	return LauncherSnapshot{
		Server: LauncherServerSnapshot{Health: options.health, Readiness: options.readiness, SystemStatus: options.systemStatus},
		Launcher: LauncherLocalSnapshot{
			ProcessID: c.process.ProcessID(), ProcessLifecycle: lifecycle, ProcessOwnership: ownership,
			EnvironmentChecks: inspection.Checks, PreflightChecks: inspection.PreflightChecks, AdvisoryChecks: inspection.AdvisoryChecks,
			RecentStderr: c.process.RecentStderr(), RuntimePrepare: options.runtimePrepare,
			ReleaseCheck: currentRelease, LastLocalError: options.lastLocalError, StatusHint: options.statusHint,
			Settings: operation.settings, ResolvedSettings: operation.resolvedSettings, Endpoint: operation.endpoint,
			LocalRecoverySummary: options.localRecoverySummary,
		},
	}
}

func (c *Coordinator) publish(snapshot LauncherSnapshot) {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	snapshot = cloneSnapshot(snapshot)
	c.mu.Lock()
	snapshot.Launcher.ReleaseCheck = cloneReleaseCheck(c.snapshot.Launcher.ReleaseCheck)
	if reflect.DeepEqual(c.snapshot, snapshot) {
		c.mu.Unlock()
		return
	}
	c.snapshot = snapshot
	c.mu.Unlock()
	c.emitSnapshot(snapshot)
}

func (c *Coordinator) emitSnapshot(snapshot LauncherSnapshot) {
	if c.host != nil {
		c.host.Emit("launcher:snapshot", snapshot)
		nextTrayState := trayState(snapshot)
		if !c.trayStateSet || c.trayState != nextTrayState {
			c.trayState = nextTrayState
			c.trayStateSet = true
			c.host.SetTrayState(nextTrayState)
		}
	}
}

func (c *Coordinator) publishRelease(snapshot ReleaseCheckSnapshot) {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.mu.Lock()
	current := cloneSnapshot(c.snapshot)
	current.Launcher.ReleaseCheck = cloneReleaseCheck(snapshot)
	if reflect.DeepEqual(c.snapshot, current) {
		c.mu.Unlock()
		return
	}
	c.snapshot = current
	c.mu.Unlock()
	c.emitSnapshot(current)
}

func (c *Coordinator) monitor(ctx context.Context) {
	statusTicker := time.NewTicker(2 * time.Second)
	releaseTicker := time.NewTicker(releaseCacheTTL)
	defer statusTicker.Stop()
	defer releaseTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-statusTicker.C:
			if !c.operationMu.TryLock() {
				continue
			}
			_ = c.refreshCurrentPassive()
			c.operationMu.Unlock()
		case <-releaseTicker.C:
			go c.refreshRelease(false)
		}
	}
}

func (c *Coordinator) refreshRelease(force bool) {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	current := c.Snapshot().Launcher.ReleaseCheck
	if current.Status == "downloading" || current.Status == "installing" {
		return
	}
	if !force && current.Status == "ready_to_install" {
		return
	}
	current.Status, current.Summary = "checking", "正在检查更新。"
	current.CanCheck, current.CanDownload, current.CanInstall = false, false, false
	c.publishRelease(current)
	c.publishRelease(c.release.GetSnapshot(force))
}

func (c *Coordinator) downloadRelease() {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	current := c.Snapshot().Launcher.ReleaseCheck
	if current.Status == "checking" || current.Status == "downloading" || current.Status == "installing" {
		return
	}
	current.Status, current.Summary = "downloading", "正在下载更新。"
	current.CanCheck, current.CanDownload, current.CanInstall = false, false, false
	c.publishRelease(current)
	c.publishRelease(c.release.Download())
}

func (c *Coordinator) quickHealthy(endpoint ServerEndpoint) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.management.IsHealthy(ctx, endpoint)
}

func (c *Coordinator) developmentWatcherActive() bool {
	return c.watcherPID > 0 && processAlive(c.watcherPID)
}

func defaultSnapshot() LauncherSnapshot {
	return LauncherSnapshot{
		Server: LauncherServerSnapshot{},
		Launcher: LauncherLocalSnapshot{
			ProcessLifecycle: "stopped", ProcessOwnership: "none", EnvironmentChecks: []EnvironmentCheckResult{}, PreflightChecks: []EnvironmentCheckResult{}, AdvisoryChecks: []EnvironmentCheckResult{}, RecentStderr: []string{},
			ReleaseCheck: releaseUnavailable("尚未检查版本。"), Settings: LauncherSettings{CloseBehavior: closeAsk}, Endpoint: ServerEndpoint{Host: "127.0.0.1", Port: 8080, BaseURL: "http://127.0.0.1:8080/"},
		},
	}
}

func cloneSnapshot(snapshot LauncherSnapshot) LauncherSnapshot {
	clone := snapshot
	clone.Server.Health = cloneJSONValue(snapshot.Server.Health)
	clone.Server.Readiness = cloneJSONValue(snapshot.Server.Readiness)
	clone.Server.SystemStatus = cloneJSONValue(snapshot.Server.SystemStatus)
	clone.Launcher.ProcessID = clonePointer(snapshot.Launcher.ProcessID)
	clone.Launcher.EnvironmentChecks = cloneSlice(snapshot.Launcher.EnvironmentChecks)
	clone.Launcher.PreflightChecks = cloneSlice(snapshot.Launcher.PreflightChecks)
	clone.Launcher.AdvisoryChecks = cloneSlice(snapshot.Launcher.AdvisoryChecks)
	clone.Launcher.RecentStderr = cloneSlice(snapshot.Launcher.RecentStderr)
	clone.Launcher.RuntimePrepare = cloneRuntimePrepare(snapshot.Launcher.RuntimePrepare)
	clone.Launcher.ReleaseCheck = cloneReleaseCheck(snapshot.Launcher.ReleaseCheck)
	clone.Launcher.Settings = cloneSettings(snapshot.Launcher.Settings)
	clone.Launcher.LocalRecoverySummary = cloneJSONValue(snapshot.Launcher.LocalRecoverySummary)
	return clone
}

func cloneSettings(settings LauncherSettings) LauncherSettings {
	clone := settings
	if settings.AdvancedOverrides != nil {
		overrides := *settings.AdvancedOverrides
		clone.AdvancedOverrides = &overrides
	}
	return clone
}

func cloneRuntimePrepare(snapshot *RuntimePrepareSnapshot) *RuntimePrepareSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.Resources = cloneSlice(snapshot.Resources)
	for index := range clone.Resources {
		resource := &clone.Resources[index]
		resource.Progress = clonePointer(resource.Progress)
		resource.DownloadedBytes = clonePointer(resource.DownloadedBytes)
		resource.TotalBytes = clonePointer(resource.TotalBytes)
		resource.ExtractedEntries = clonePointer(resource.ExtractedEntries)
		resource.TotalEntries = clonePointer(resource.TotalEntries)
	}
	return &clone
}

func cloneReleaseCheck(snapshot ReleaseCheckSnapshot) ReleaseCheckSnapshot {
	clone := snapshot
	clone.DownloadProgress = clonePointer(snapshot.DownloadProgress)
	clone.DownloadedBytes = clonePointer(snapshot.DownloadedBytes)
	clone.TotalBytes = clonePointer(snapshot.TotalBytes)
	return clone
}

func cloneJSONValue(value any) any {
	switch current := value.(type) {
	case JSONObject:
		clone := make(JSONObject, len(current))
		for key, item := range current {
			clone[key] = cloneJSONValue(item)
		}
		return clone
	case map[string]any:
		clone := make(map[string]any, len(current))
		for key, item := range current {
			clone[key] = cloneJSONValue(item)
		}
		return clone
	case []any:
		clone := make([]any, len(current))
		for index, item := range current {
			clone[index] = cloneJSONValue(item)
		}
		return clone
	default:
		return current
	}
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	return append(make([]T, 0, len(values)), values...)
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func trayState(snapshot LauncherSnapshot) TrayMenuState {
	state := "未启动"
	canOpen := snapshot.Server.Health != nil && snapshot.Server.Readiness != nil
	if snapshot.Launcher.ProcessLifecycle == "starting" {
		state = "启动中"
	} else if snapshot.Launcher.ProcessLifecycle == "stopping" {
		state = "停止中"
	} else if canOpen {
		switch readinessStatus(snapshot.Server.Readiness) {
		case "ready", "setup_required":
			state = "运行中"
		case "degraded":
			state = "运行条件受限"
		default:
			state = "启动失败"
		}
	} else if snapshot.Launcher.LastLocalError != "" {
		state = "启动失败"
	}
	action, label := "start", "启动服务"
	canRun := snapshot.Launcher.ProcessLifecycle != "starting" && snapshot.Launcher.ProcessLifecycle != "stopping"
	if snapshot.Launcher.ProcessOwnership != "none" {
		action, label = "stop", "停止服务"
	}
	return TrayMenuState{TrayStatusSummary: state, CanOpenWebUI: canOpen, TrayServiceAction: action, TrayServiceActionLabel: label, CanRunTrayServiceAction: canRun}
}

func endpointListening(endpoint ServerEndpoint) bool {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)), 400*time.Millisecond)
	if err != nil {
		return false
	}
	connection.Close()
	return true
}

func lifecycleFor(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func serviceAvailable(snapshot LauncherSnapshot) bool {
	if objectStatusFromAny(snapshot.Server.Health) != "ok" {
		return false
	}
	switch readinessStatus(snapshot.Server.Readiness) {
	case "ready", "degraded", "setup_required":
		return true
	default:
		return false
	}
}

func ownershipFor(managed, reachable bool) string {
	if managed {
		return "launcher_managed"
	}
	if reachable {
		return "external"
	}
	return "none"
}

func readinessStatus(value any) string {
	switch payload := value.(type) {
	case JSONObject:
		status, _ := payload["status"].(string)
		return status
	case map[string]any:
		status, _ := payload["status"].(string)
		return status
	default:
		return ""
	}
}

func recoveryFromPayload(systemStatus any, readiness JSONObject, fallback any) any {
	switch payload := systemStatus.(type) {
	case JSONObject:
		if payload["recovery_summary"] != nil {
			return payload["recovery_summary"]
		}
	case map[string]any:
		if payload["recovery_summary"] != nil {
			return payload["recovery_summary"]
		}
	}
	if readiness["recovery_summary"] != nil {
		return readiness["recovery_summary"]
	}
	return fallback
}

func sanitizeWebTargetPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n\x00") {
		return "", errors.New("管理界面目标必须是安全的站内绝对路径")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return "", errors.New("管理界面目标路径无效")
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		if segment == "." || segment == ".." || strings.EqualFold(segment, "%2e") || strings.EqualFold(segment, "%2e%2e") {
			return "", errors.New("管理界面目标路径不能包含目录跳转")
		}
	}
	return parsed.String(), nil
}

func primaryEnvironmentIssue(checks []EnvironmentCheckResult) *EnvironmentCheckResult {
	for index := range checks {
		if checks[index].Severity == "error" {
			return &checks[index]
		}
	}
	for index := range checks {
		if checks[index].Severity == "warning" {
			return &checks[index]
		}
	}
	return nil
}

func environmentIssueDetail(inspection EnvironmentInspection) string {
	if issue := primaryEnvironmentIssue(inspection.PreflightChecks); issue != nil {
		return strings.TrimSpace(issue.Summary + " " + issue.Detail + " " + issue.Remediation)
	}
	return "启动器预检发现阻塞项。"
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
