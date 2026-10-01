package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
	if inspection.HasBlockingIssues || inspection.CanBootstrapUserConfig {
		lifecycle := Stopped
		ownership := OwnershipNone
		if c.process.IsRunning() {
			lifecycle = "running"
			ownership = "launcher_managed"
		}
		// A missing user configuration is generated on start, so only a blocking issue needs a hint here.
		hint := ""
		if issue := primaryEnvironmentIssue(inspection.PreflightChecks); issue != nil && inspection.HasBlockingIssues {
			hint = issue.Summary + " " + issue.Remediation
		}
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			processLifecycle: lifecycle, processOwnership: ownership, statusHint: strings.TrimSpace(hint),
		}))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), refreshRequestBudget)
	defer cancel()
	healthy := c.management.IsHealthy(ctx, operation.endpoint)
	// Read status regardless of readiness: shutdown is independent of database
	// readiness and may start while setup or resource recovery is in progress.
	var systemStatus *ServerSystemStatusResponse
	var statusErr error
	if healthy || c.process.IsRunning() {
		systemStatus, statusErr = c.management.GetLauncherStatus(ctx, operation.endpoint)
		if statusErr == nil {
			c.process.RememberShutdownBudget(systemStatus.ShutdownBudgetSeconds)
			if systemStatus.Status == "shutting_down" {
				c.process.MarkStopping()
			}
		}
	}
	if !healthy {
		lifecycle := Stopped
		ownership := OwnershipNone
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
			processLifecycle: lifecycle, processOwnership: ownership, statusHint: hint, lastLocalError: lastError,
		}))
		return nil
	}

	readiness, err := c.management.GetReadiness(ctx, operation.endpoint)
	if err != nil {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
			health: &ServerLivenessStatusResponse{Status: "ok"}, processLifecycle: lifecycleFor(c.process.IsRunning()), processOwnership: ownershipFor(c.process.IsRunning(), true),
			statusHint: "服务存活，但无法读取就绪状态。", lastLocalError: err.Error(),
		}))
		return nil
	}
	statusError, statusHint := "", ""
	if statusErr != nil {
		statusHint, statusError = describeLauncherStatusError(statusErr, c.process.IsRunning())
	}
	lifecycle := lifecycleFor(c.process.IsRunning())
	if systemStatus != nil && systemStatus.Status == "shutting_down" {
		lifecycle = "stopping"
	}
	c.process.ClearRuntimePrepare()
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		health: &ServerLivenessStatusResponse{Status: "ok"}, readiness: readiness, systemStatus: systemStatus,
		processLifecycle: lifecycle, processOwnership: ownershipFor(c.process.IsRunning(), true),
		statusHint: statusHint, lastLocalError: statusError,
	}))
	return nil
}

// A service started outside this launcher does not hold its control token, so the launcher endpoints refuse it.
// That is expected rather than a fault: the service keeps running and is stopped from the management UI.
func describeLauncherStatusError(err error, launcherManaged bool) (hint, lastError string) {
	var serverErr *ServerError
	if !launcherManaged && errors.As(err, &serverErr) && serverErr.StatusCode == http.StatusForbidden {
		return "这个服务不是由启动器启动的，启动器无法停止它；需要停止时请在管理界面停止服务。", ""
	}
	return "", err.Error()
}

func (c *Coordinator) Start() error {
	startupContext, finishStartup, allowed := c.startups.begin()
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
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "无法生成用户配置。", lastLocalError: "配置初始化命令已完成，但仍未生成用户配置。"}))
			return nil
		}
	}
	if inspection.HasBlockingIssues {
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: environmentIssueDetail(inspection)}))
		return nil
	}
	ctx, cancel := context.WithTimeout(startupContext, startHealthProbeBudget)
	healthy := c.management.IsHealthy(ctx, operation.endpoint)
	cancel()
	if startupContext.Err() != nil {
		return nil
	}
	if healthy {
		return c.refresh(operation)
	}
	if c.developmentWatcherActive() && !c.process.IsRunning() {
		c.process.WriteLauncherLog("服务已由开发脚本管理，无需重复启动。", operation.resolvedSettings.Workdir)
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

	deadline := time.Now().Add(startupReadinessBudget)
	failedSince := time.Time{}
	var lastFailed *ServerReadinessStatusResponse
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
		ctx, cancel := context.WithTimeout(startupContext, startupProbeTimeout)
		if c.management.IsHealthy(ctx, operation.endpoint) {
			if readiness, readErr := c.management.GetReadiness(ctx, operation.endpoint); readErr == nil {
				if readiness.Status == "failed" {
					lastFailed = readiness
					if failedSince.IsZero() {
						failedSince = time.Now()
					}
					if time.Since(failedSince) < startupFailureStabilization {
						cancel()
						if !waitForStartup(startupContext, startupPollInterval) {
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
		if !waitForStartup(startupContext, startupPollInterval) {
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
	unblockStartups := c.startups.block(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	err := c.stopLocked(true, shutdownIntentStop)
	if errors.Is(err, errStopCancelled) {
		return nil
	}
	return err
}

func (c *Coordinator) Restart() error {
	unblockStartups := c.startups.block(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if !c.process.IsRunning() {
		return errors.New("只能重启由启动器管理的服务")
	}
	if err := c.stopLocked(true, shutdownIntentRestart); err != nil {
		return err
	}
	// Retain the operation lock across stop and start. Other stop/exit requests
	// keep their own blockers and may reject or cancel the new startup.
	unblockStartups()
	startupContext, finishStartup, allowed := c.startups.begin()
	if !allowed {
		return fmt.Errorf("服务已停止，但重启被阻止: %w", errStartupBlocked)
	}
	defer finishStartup()
	return c.startLocked(startupContext)
}

func (c *Coordinator) stopLocked(confirmExternal bool, intent shutdownIntent) error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	inspection := InspectEnvironment(operation.resolvedSettings)
	healthy := c.quickHealthy(operation.endpoint)
	managed := c.process.IsRunning()
	ownership := OwnershipNone
	if managed {
		ownership = "launcher_managed"
	} else if healthy {
		ownership = "external"
	}
	if ownership == "external" && confirmExternal {
		// Publish capabilities before asking, including when the first stop
		// happens before a status refresh has discovered this external service.
		if err := c.refresh(operation); err != nil {
			return err
		}
		if c.host == nil || !c.host.ConfirmExternalServiceStop() {
			return errStopCancelled
		}
	}
	if ownership == "external" {
		snapshot := c.Snapshot()
		snapshot.Launcher.StatusHint = "请在管理界面停止现有服务，停止后再重试。"
		snapshot.Launcher.LastLocalError = ""
		c.publish(snapshot)
		if c.host != nil {
			if err := c.OpenWebUI(""); err != nil {
				return err
			}
		}
		return &BoundaryError{Code: "launcher.external_stop_required", Message: "请先在管理界面停止现有服务。"}
	}
	if managed {
		c.process.MarkStopping()
	}
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		health: func() *ServerLivenessStatusResponse {
			if healthy {
				return &ServerLivenessStatusResponse{Status: "ok"}
			}
			return nil
		}(),
		processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止服务。",
	}))
	if managed {
		err := stopManagedProcess(c.process, func() error {
			if !healthy {
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), shutdownRequestTimeout)
			defer cancel()
			return c.management.Shutdown(ctx, operation.endpoint, intent)
		}, c.process.ShutdownWaitBudget())
		if err != nil {
			return err
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
		ownership := OwnershipExternal
		if managed {
			ownership = "launcher_managed"
		}
		c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止服务以重置管理员账号。"}))
		if managed {
			if err := c.process.ForceKill(); err != nil {
				return err
			}
		} else {
			if !isLoopbackHost(operation.endpoint.Host) {
				return errors.New("无法重置非本机服务的管理员账号")
			}
			stopped, stopErr := stopEndpointProcess(operation.endpoint)
			if stopErr != nil {
				return stopErr
			}
			if !stopped {
				return errors.New("无法确认现有服务进程，管理员账号未重置")
			}
		}
	}
	if err := c.process.RunOffline(operation.resolvedSettings, "reset-admin"); err != nil {
		return err
	}
	startupContext, finishStartup, allowed := c.startups.begin()
	if !allowed {
		return fmt.Errorf("管理员账号已重置，但服务重启被阻止: %w", errStartupBlocked)
	}
	defer finishStartup()
	if err := c.startLocked(startupContext); err != nil {
		return err
	}
	deadline := time.Now().Add(resetAdminReadinessBudget)
	for time.Now().Before(deadline) {
		if startupContext.Err() != nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(startupContext, resetAdminProbeTimeout)
		readiness, readErr := c.management.GetReadiness(ctx, operation.endpoint)
		cancel()
		if readErr == nil && readiness.Status == "setup_required" {
			return c.OpenWebUI("")
		}
		if !waitForStartup(startupContext, startupPollInterval) {
			return nil
		}
	}
	return errors.New("管理员账号已重置，但服务未在预期时间内进入初始化流程")
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

func (c *Coordinator) quickHealthy(endpoint ServerEndpoint) bool {
	ctx, cancel := context.WithTimeout(context.Background(), quickHealthProbeTimeout)
	defer cancel()
	return c.management.IsHealthy(ctx, endpoint)
}

func (c *Coordinator) developmentWatcherActive() bool {
	return c.watcherPID > 0 && processAlive(c.watcherPID)
}

func endpointListening(endpoint ServerEndpoint) bool {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)), endpointConnectTimeout)
	if err != nil {
		return false
	}
	connection.Close()
	return true
}
