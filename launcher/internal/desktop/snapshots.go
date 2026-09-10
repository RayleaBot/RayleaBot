package desktop

import (
	"strings"
)

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
