package app

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func (s *systemService) activePluginCount() int {
	if s == nil || s.runtimes == nil {
		return 0
	}
	return s.runtimes.ActiveCount()
}

func (s *systemService) uptimeSeconds() int64 {
	if s == nil || s.state == nil || s.state.startedAt.IsZero() {
		return 0
	}

	uptime := time.Since(s.state.startedAt)
	if uptime < 0 {
		return 0
	}

	return int64(uptime / time.Second)
}

func (s *systemService) systemStatus() string {
	if s != nil && s.shuttingDown != nil && s.shuttingDown.Load() {
		return "shutting_down"
	}
	return "running"
}

func (s *systemService) publishStatusSnapshot() {
	if s == nil || s.statusPublisher == nil {
		return
	}
	s.statusPublisher.PublishSnapshot()
}

func recoveryIssuesToHealth(issues []recovery.CompatibilityIssue) []health.DiagnosticIssue {
	if len(issues) == 0 {
		return nil
	}
	items := make([]health.DiagnosticIssue, 0, len(issues))
	for _, issue := range issues {
		items = append(items, health.DiagnosticIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return items
}

func containsRuntimeKind(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
