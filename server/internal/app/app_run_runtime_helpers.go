package app

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func (s *appRuntimeState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}

func (s *appRuntimeState) recoverySummarySnapshot() *recovery.CompatibilitySummary {
	if s == nil {
		return nil
	}
	s.recoveryMu.RLock()
	defer s.recoveryMu.RUnlock()
	if s.recoverySummary == nil {
		return nil
	}
	summary := *s.recoverySummary
	if summary.UpdatedAt == "" {
		summary.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return &summary
}

func (s *appRuntimeState) setRecoverySummary(summary *recovery.CompatibilitySummary) {
	if s == nil {
		return
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	s.recoverySummary = summary
}
