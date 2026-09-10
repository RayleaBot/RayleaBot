package system

import (
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type ReadinessReport struct {
	Status          string                         `json:"status"`
	Reason          string                         `json:"reason,omitempty"`
	ReasonCodes     []string                       `json:"reason_codes,omitempty"`
	Checks          map[string]string              `json:"checks,omitempty"`
	Issues          []health.DiagnosticIssue       `json:"issues,omitempty"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}
