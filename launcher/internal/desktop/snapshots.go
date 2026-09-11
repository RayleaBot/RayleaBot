package desktop

import (
	"encoding/json"
	"strings"
)

func defaultSnapshot() LauncherSnapshot {
	return LauncherSnapshot{
		Server: LauncherServerSnapshot{},
		Launcher: LauncherLocalSnapshot{
			ProcessLifecycle: "stopped", ProcessOwnership: "none", EnvironmentChecks: []EnvironmentCheckResult{}, PreflightChecks: []EnvironmentCheckResult{}, AdvisoryChecks: []EnvironmentCheckResult{}, RecentStderr: []string{},
			ReleaseCheck: releaseUnavailable("尚未检查版本。"), Settings: LauncherSettings{CloseBehavior: CloseAskEveryTime}, Endpoint: ServerEndpoint{Host: "127.0.0.1", Port: 8080, BaseURL: "http://127.0.0.1:8080/"},
		},
	}
}

func cloneSnapshot(snapshot LauncherSnapshot) LauncherSnapshot {
	clone := snapshot
	clone.Server.Health = cloneResponse(snapshot.Server.Health)
	clone.Server.Readiness = cloneResponse(snapshot.Server.Readiness)
	clone.Server.SystemStatus = cloneResponse(snapshot.Server.SystemStatus)
	clone.Launcher.ProcessID = clonePointer(snapshot.Launcher.ProcessID)
	clone.Launcher.EnvironmentChecks = cloneSlice(snapshot.Launcher.EnvironmentChecks)
	clone.Launcher.PreflightChecks = cloneSlice(snapshot.Launcher.PreflightChecks)
	clone.Launcher.AdvisoryChecks = cloneSlice(snapshot.Launcher.AdvisoryChecks)
	clone.Launcher.RecentStderr = cloneSlice(snapshot.Launcher.RecentStderr)
	clone.Launcher.RuntimePrepare = cloneRuntimePrepare(snapshot.Launcher.RuntimePrepare)
	clone.Launcher.ReleaseCheck = cloneReleaseCheck(snapshot.Launcher.ReleaseCheck)
	clone.Launcher.Settings = cloneSettings(snapshot.Launcher.Settings)
	clone.Launcher.LocalRecoverySummary = cloneResponse(snapshot.Launcher.LocalRecoverySummary)
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

func cloneResponse[T any](value *T) *T {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return &result
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

func lifecycleFor(running bool) LauncherProcessLifecycle {
	if running {
		return "running"
	}
	return "stopped"
}

func serviceAvailable(snapshot LauncherSnapshot) bool {
	if snapshot.Server.Health == nil || snapshot.Server.Health.Status != "ok" {
		return false
	}
	switch readinessStatus(snapshot.Server.Readiness) {
	case "ready", "degraded", "setup_required":
		return true
	default:
		return false
	}
}

func ownershipFor(managed, reachable bool) LauncherProcessOwnership {
	if managed {
		return "launcher_managed"
	}
	if reachable {
		return "external"
	}
	return "none"
}

func readinessStatus(value *ServerReadinessStatusResponse) string {
	if value == nil {
		return ""
	}
	return value.Status
}

func recoveryFromPayload(systemStatus *ServerSystemStatusResponse, readiness *ServerReadinessStatusResponse, fallback *ServerRecoveryCompatibilitySummary) *ServerRecoveryCompatibilitySummary {
	if systemStatus != nil && systemStatus.RecoverySummary != nil {
		return systemStatus.RecoverySummary
	}
	if readiness != nil && readiness.RecoverySummary != nil {
		return readiness.RecoverySummary
	}
	return fallback
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
