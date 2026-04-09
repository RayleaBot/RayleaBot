package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (a *App) currentReadiness() health.ReadinessReport {
	if a == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "Management application is unavailable",
			Checks: map[string]string{
				"config": "unknown", "database": "unknown", "runtime": "unknown", "adapter": "unknown", "render": "unknown",
			},
			Issues: []health.DiagnosticIssue{
				{
					Code:        "management.unavailable",
					Severity:    "error",
					Summary:     "Management application is unavailable",
					Remediation: "请检查服务进程是否已正确启动。",
				},
			},
			RecoverySummary: nil,
		}
	}
	if a.Auth == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "Management auth service is unavailable",
			Checks: map[string]string{
				"config": "ok", "database": "unknown", "runtime": "unknown", "adapter": "unknown", "render": "unknown",
			},
			Issues: []health.DiagnosticIssue{
				{
					Code:        "auth.unavailable",
					Severity:    "error",
					Summary:     "Management auth service is unavailable",
					Remediation: "请检查服务日志，确认认证服务已完成初始化。",
				},
			},
			RecoverySummary: a.recoverySummary,
		}
	}
	if !a.Auth.IsBootstrapped() {
		return health.ReadinessReport{
			Status: "setup_required",
			Reason: "Initial admin setup is required",
			Checks: map[string]string{
				"config": "ok",
			},
			Issues: []health.DiagnosticIssue{
				{
					Code:        "setup.required",
					Severity:    "error",
					Summary:     "Initial admin setup is required",
					Remediation: "请先完成管理员初始化，然后再使用管理入口。",
				},
			},
			RecoverySummary: a.recoverySummary,
		}
	}
	if a.Adapter == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "OneBot adapter is unavailable",
			Checks: map[string]string{
				"config": "ok", "database": "ok", "runtime": "ok", "adapter": "unavailable", "render": "ok",
			},
			Issues: []health.DiagnosticIssue{
				{
					Code:        "adapter.unavailable",
					Severity:    "error",
					Summary:     "OneBot adapter is unavailable",
					Remediation: "请检查 OneBot adapter 配置并重启服务。",
				},
			},
			RecoverySummary: a.recoverySummary,
		}
	}

	report := ReadinessReportFromAdapter(a.Adapter.Snapshot())
	report.RecoverySummary = a.recoverySummary
	if a.recoverySummary != nil {
		switch a.recoverySummary.Status {
		case "blocked":
			report.Status = "failed"
			report.Reason = "Recovery compatibility checks blocked startup"
			report.ReasonCodes = []string{"recovery.blocked"}
			report.Checks["runtime"] = "recovery_blocked"
		case "degraded", "pending":
			if report.Status == "ready" {
				report.Status = "degraded"
				report.Reason = "Recovery compatibility checks require attention"
				report.ReasonCodes = []string{"recovery.degraded"}
			}
		}
		report.Issues = append(report.Issues, recoveryIssuesToHealth(a.recoverySummary.Issues)...)
	}
	pluginsList := []plugins.Snapshot(nil)
	if a.Plugins != nil {
		pluginsList = a.Plugins.List()
	}

	renderIssues := recoveryIssuesToHealth(a.renderDiagnostics())
	if len(renderIssues) > 0 {
		report.Checks["render"] = "resource_missing"
		report.Issues = append(report.Issues, renderIssues...)
	}

	runtimeIssues := recoveryIssuesToHealth(a.managedRuntimeDiagnostics(pluginsList))
	if len(runtimeIssues) > 0 {
		report.Checks["runtime"] = "resource_missing"
		report.Issues = append(report.Issues, runtimeIssues...)
	}

	if report.Status == "ready" && (len(renderIssues) > 0 || len(runtimeIssues) > 0) {
		reason := "运行环境需要处理"
		if len(runtimeIssues) > 0 {
			reason = runtimeIssues[0].Summary
		} else if len(renderIssues) > 0 {
			reason = renderIssues[0].Summary
		}
		report.Status = "degraded"
		report.Reason = reason
		report.ReasonCodes = []string{"platform.resource_missing"}
	}
	return report
}

func ReadinessReportFromAdapter(snapshot adapter.Snapshot) health.ReadinessReport {
	report := health.ReadinessReport{
		Checks: map[string]string{
			"config":   "ok",
			"database": "ok",
			"runtime":  "ok",
			"adapter":  "ok",
			"render":   "ok",
		},
	}

	switch stateOrIdle(snapshot.State) {
	case adapter.StateConnected:
		report.Status = "ready"
		report.Checks["adapter"] = "ok"
	case adapter.StateIdle:
		report.Status = "ready"
		report.Checks["adapter"] = "idle"
	case adapter.StateAuthFailed:
		report.Status = "ready"
		report.Checks["adapter"] = "auth_failed"
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        firstNonEmpty(snapshot.LastErrorCode, "adapter.auth_failed"),
			Severity:    "warning",
			Summary:     "OneBot 鉴权失败",
			Remediation: "请检查 OneBot access_token 配置后重试连接。",
		})
	case adapter.StateReconnecting:
		report.Status = "ready"
		report.Checks["adapter"] = "reconnecting"
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        firstNonEmpty(snapshot.LastErrorCode, "adapter.connection_lost"),
			Severity:    "warning",
			Summary:     "OneBot 正在重连",
			Remediation: "请检查 OneBot 服务可用性，或等待连接自动恢复。",
		})
	case adapter.StateConnecting:
		report.Status = "degraded"
		report.Checks["adapter"] = "connecting"
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        "adapter.connection_pending",
			Severity:    "warning",
			Summary:     "OneBot 正在建立连接",
			Remediation: "请稍后重试，或检查上游服务是否可达。",
		})
	default:
		report.Status = "failed"
		report.Checks["adapter"] = "failed"
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        "adapter.connection_failed",
			Severity:    "error",
			Summary:     "OneBot 传输链路不可用",
			Remediation: "请检查 OneBot 传输配置、访问令牌和上游服务状态。",
		})
	}

	appendIssue := func(code, message string) {
		if code == "" {
			return
		}
		severity := "warning"
		if report.Status == "failed" {
			severity = "error"
		}
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        code,
			Severity:    severity,
			Summary:     message,
			Remediation: "请检查协议中心中的 OneBot 传输状态与日志。",
		})
	}
	appendIssue(snapshot.ForwardWS.LastErrorCode, snapshot.ForwardWS.LastErrorMessage)
	appendIssue(snapshot.ReverseWS.LastErrorCode, snapshot.ReverseWS.LastErrorMessage)
	appendIssue(snapshot.HTTPAPI.LastErrorCode, snapshot.HTTPAPI.LastErrorMessage)
	appendIssue(snapshot.Webhook.LastErrorCode, snapshot.Webhook.LastErrorMessage)

	return report
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stateOrIdle(state adapter.State) adapter.State {
	if state == "" {
		return adapter.StateIdle
	}
	return state
}
