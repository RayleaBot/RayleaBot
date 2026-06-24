package system

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func (s *Service) activePluginCount() int {
	if s == nil || s.runtimes == nil {
		return 0
	}
	return s.runtimes.ActiveCount()
}

func (s *Service) pluginStateCounts() (running int, failed int) {
	if s == nil || s.plugins == nil {
		return 0, 0
	}
	for _, snapshot := range s.plugins.List() {
		state, _ := plugins.ProjectState(snapshot)
		switch state {
		case plugins.PluginStateRunning:
			running++
		case plugins.PluginStateFailed:
			failed++
		}
	}
	return running, failed
}

func (s *Service) dbSchemaVersion() string {
	return storage.CurrentSchemaVersion()
}

func (s *Service) uptimeSeconds() int64 {
	startedAt := s.startedAtValue()
	if startedAt.IsZero() {
		return 0
	}

	uptime := time.Since(startedAt)
	if uptime < 0 {
		return 0
	}

	return int64(uptime / time.Second)
}

func (s *Service) systemStatus() string {
	if s != nil && s.shuttingDown != nil && s.shuttingDown.Load() {
		return "shutting_down"
	}
	return "running"
}

func (s *Service) PublishStatusSnapshot() {
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
