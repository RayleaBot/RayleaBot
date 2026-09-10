package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
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
	recovery, recoveryErr := readRecoverySummary(c.process.LogDirectory())
	if recoveryErr != nil {
		check := EnvironmentCheckResult{Scope: "advisory", Code: "launcher.recovery_summary_invalid", Title: "本机恢复摘要", Severity: "warning", Summary: recoveryErr.Error(), Remediation: "检查 logs/recovery-summary.json；服务可用后以服务端摘要为准。"}
		inspection.AdvisoryChecks = append(inspection.AdvisoryChecks, check)
		inspection.Checks = append(inspection.Checks, check)
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), refreshRequestBudget)
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
			health: &ServerLivenessStatusResponse{Status: "ok"}, processLifecycle: lifecycleFor(c.process.IsRunning()), processOwnership: ownershipFor(c.process.IsRunning(), true),
			statusHint: "服务存活，但无法读取就绪状态。", lastLocalError: err.Error(), localRecoverySummary: recovery,
		}))
		return nil
	}
	var systemStatus *ServerSystemStatusResponse
	statusError := ""
	if status := readiness.Status; status == "ready" || status == "degraded" {
		if value, statusErr := c.management.GetLauncherStatus(ctx, operation.endpoint); statusErr == nil {
			systemStatus = value
		} else {
			statusError = statusErr.Error()
		}
	}
	recovery = recoveryFromPayload(systemStatus, readiness, recovery)
	lifecycle := lifecycleFor(c.process.IsRunning())
	if systemStatus != nil && systemStatus.Status == "shutting_down" {
		lifecycle = "stopping"
	}
	c.process.ClearRuntimePrepare()
	c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{
		health: &ServerLivenessStatusResponse{Status: "ok"}, readiness: readiness, systemStatus: systemStatus,
		processLifecycle: lifecycle, processOwnership: ownershipFor(c.process.IsRunning(), true), localRecoverySummary: recovery,
		lastLocalError: statusError,
	}))
	return nil
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
			c.publish(c.buildSnapshot(operation, inspection, snapshotOptions{processLifecycle: "stopped", processOwnership: "none", statusHint: "无法生成用户配置。", lastLocalError: "配置初始化命令已完成，但 config/user.yaml 仍未生成。"}))
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
			health: &ServerLivenessStatusResponse{Status: "ok"}, processLifecycle: "stopping", processOwnership: ownership, statusHint: "正在停止现有服务。",
		}))
		ctx, cancel := context.WithTimeout(context.Background(), shutdownRequestTimeout)
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
			return c.management.Shutdown(ctx, operation.endpoint)
		}, stopGracePeriod)
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
	startupContext, finishStartup, allowed := c.startups.begin()
	if !allowed {
		return fmt.Errorf("管理员凭据已重置，但服务重启被阻止: %w", errStartupBlocked)
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
	return errors.New("管理员凭据已重置，但服务未在预期时间内进入 setup_required 状态")
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
