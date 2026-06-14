package system

import (
	"strings"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (s *Service) CurrentReadiness() health.ReadinessReport {
	if s == nil {
		return normalizeReadinessReport(health.ReadinessReport{
			Status: "failed",
			Reason: "Management application is unavailable",
			Checks: map[string]string{
				"config": "unknown", "database": "unknown", "runtime": "unknown", "render": "unknown",
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
		})
	}
	if s.auth == nil {
		return normalizeReadinessReport(health.ReadinessReport{
			Status: "failed",
			Reason: "Management auth service is unavailable",
			Checks: map[string]string{
				"config": "ok", "database": "unknown", "runtime": "unknown", "render": "unknown",
			},
			Issues: []health.DiagnosticIssue{
				{
					Code:        "auth.unavailable",
					Severity:    "error",
					Summary:     "Management auth service is unavailable",
					Remediation: "请检查服务日志，确认认证服务已完成初始化。",
				},
			},
			RecoverySummary: s.recoverySummarySnapshot(),
		})
	}
	if !s.auth.IsBootstrapped() {
		return normalizeReadinessReport(health.ReadinessReport{
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
			RecoverySummary: s.recoverySummarySnapshot(),
		})
	}
	report := health.ReadinessReport{
		Status: "ready",
		Checks: map[string]string{
			"config":   "ok",
			"database": "ok",
			"runtime":  "ok",
			"render":   "ok",
		},
	}
	report.RecoverySummary = s.recoverySummarySnapshot()
	if report.RecoverySummary != nil {
		switch report.RecoverySummary.Status {
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
		report.Issues = append(report.Issues, recoveryIssuesToHealth(report.RecoverySummary.Issues)...)
	}
	pluginsList := []plugins.Snapshot(nil)
	if s.plugins != nil {
		pluginsList = s.plugins.List()
	}

	renderIssues := recoveryIssuesToHealth(s.renderDiagnostics())
	if len(renderIssues) > 0 {
		report.Checks["render"] = "resource_missing"
		report.Issues = append(report.Issues, renderIssues...)
	}

	runtimeIssues := recoveryIssuesToHealth(s.managedRuntimeDiagnostics(pluginsList))
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
	return normalizeReadinessReport(report)
}

func normalizeReadinessReport(report health.ReadinessReport) health.ReadinessReport {
	if report.Status != "degraded" && report.Status != "failed" {
		return report
	}
	if strings.TrimSpace(report.Reason) != "" {
		return report
	}
	if len(report.Issues) == 0 {
		return report
	}
	report.Reason = report.Issues[0].Summary
	if len(report.ReasonCodes) == 0 && strings.TrimSpace(report.Issues[0].Code) != "" {
		report.ReasonCodes = []string{report.Issues[0].Code}
	}
	return report
}

func stateOrIdle(state adaptershell.State) adaptershell.State {
	if state == "" {
		return adaptershell.StateIdle
	}
	return state
}
