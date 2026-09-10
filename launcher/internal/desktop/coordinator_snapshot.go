package desktop

import (
	"reflect"
)

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
