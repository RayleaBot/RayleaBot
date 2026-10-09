package system

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
)

func (s *Service) CurrentReadiness() ReadinessReport {
	if s.auth == nil {
		return normalizeReadinessReport(ReadinessReport{
			Status: "failed",
			Reason: "管理认证服务不可用",
			Issues: []health.DiagnosticIssue{
				{
					Code:        errorcodes.DiagnosticAuthUnavailable,
					Severity:    "error",
					Summary:     "管理认证服务不可用",
					Remediation: "请检查服务日志，确认认证服务已完成初始化。",
				},
			},
		})
	}
	if !s.auth.IsBootstrapped() {
		return normalizeReadinessReport(ReadinessReport{
			Status: "setup_required",
			Reason: "需要先完成管理员初始化",
			Issues: []health.DiagnosticIssue{
				{
					Code:        errorcodes.DiagnosticSetupRequired,
					Severity:    "error",
					Summary:     "需要先完成管理员初始化",
					Remediation: "请先完成管理员初始化，然后再使用管理入口。",
				},
			},
		})
	}
	report := ReadinessReport{
		Status: "ready",
		Checks: map[string]string{
			"database": "ok",
			"runtime":  "ok",
			"render":   "ok",
		},
	}
	if !s.databaseAvailable() {
		report.Checks["database"] = "unavailable"
		report.Status = "failed"
		report.Reason = "数据库不可用"
		report.ReasonCodes = []string{errorcodes.DiagnosticDatabasePingFailed}
		report.Issues = append(report.Issues, health.DiagnosticIssue{
			Code:        errorcodes.DiagnosticDatabasePingFailed,
			Severity:    "error",
			Summary:     "数据库不可用",
			Remediation: "请检查数据库文件、磁盘空间与文件权限，然后重启服务。",
		})
	}

	runtimeState, ok := s.startupRuntimeState("ffmpeg")
	switch {
	case ok && (runtimeState.Phase == StartupRuntimePhaseReady || runtimeState.Phase == StartupRuntimePhaseNotRequired):
	case ok && runtimeState.Phase == StartupRuntimePhasePending:
		report.Checks["runtime"] = "preparing"
	default:
		report.Checks["runtime"] = "resource_missing"
		issue := runtimeState.Issue
		if issue == nil {
			missing := startupFailureIssue("ffmpeg", nil)
			issue = &missing
		}
		report.Issues = append(report.Issues, *issue)
	}

	renderIssues := s.renderDiagnostics()
	if len(renderIssues) > 0 {
		report.Checks["render"] = "resource_missing"
		report.Issues = append(report.Issues, renderIssues...)
	}
	if report.Checks["runtime"] == "resource_missing" || len(renderIssues) > 0 {
		report.ReasonCodes = append(report.ReasonCodes, errorcodes.PlatformResourceMissing)
		if report.Status != "failed" {
			report.Status = "degraded"
			report.Reason = report.Issues[0].Summary
		}
	}
	if issue := s.inboundTokenIssue(); issue != nil {
		report.Issues = append(report.Issues, *issue)
		report.ReasonCodes = append(report.ReasonCodes, issue.Code)
		if report.Status != "failed" {
			report.Status = "degraded"
			report.Reason = report.Issues[0].Summary
		}
	}
	return normalizeReadinessReport(report)
}

func (s *Service) inboundTokenIssue() *health.DiagnosticIssue {
	if !s.requireInboundToken {
		return nil
	}
	for _, adapter := range s.config().Adapters {
		if !adapter.Enabled || adapter.Type != config.AdapterTypeOneBot11 || adapter.OneBot11 == nil {
			continue
		}
		for _, transport := range []config.OneBotTransportConfig{adapter.OneBot11.ReverseWS, adapter.OneBot11.Webhook} {
			if transport.Enabled && strings.TrimSpace(transport.AccessToken) == "" {
				return &health.DiagnosticIssue{
					Code:        errorcodes.DiagnosticAdapterInboundTokenMissing,
					Severity:    "warning",
					Summary:     "OneBot 入站未设置访问令牌",
					Remediation: "为反向 WebSocket 与 Webhook 设置访问令牌，或把 server.host 改回 127.0.0.1 后重启服务。",
				}
			}
		}
	}
	return nil
}

func (s *Service) databaseAvailable() bool {
	if s.storage == nil || s.storage.Read == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var result int
	return s.storage.Read.QueryRowContext(ctx, "SELECT 1").Scan(&result) == nil && result == 1
}

func normalizeReadinessReport(report ReadinessReport) ReadinessReport {
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
