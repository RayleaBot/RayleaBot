package health

import (
	"encoding/json"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type LivenessResponse struct {
	Status string `json:"status"`
}

type ReadinessReport struct {
	Status          string                         `json:"status"`
	Reason          string                         `json:"reason,omitempty"`
	ReasonCodes     []string                       `json:"reason_codes,omitempty"`
	Checks          map[string]string              `json:"checks,omitempty"`
	Issues          []DiagnosticIssue              `json:"issues,omitempty"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

type DiagnosticIssue struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Summary        string `json:"summary"`
	UserMessage    string `json:"user_message,omitempty"`
	Remediation    string `json:"remediation"`
	InternalReason string `json:"internal_reason,omitempty"`
}

func NewLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, LivenessResponse{Status: "ok"})
	}
}

func NewReadinessHandler(getReport func() ReadinessReport) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		report := getReport()
		statusCode := http.StatusServiceUnavailable
		if report.Status == "ready" || report.Status == "degraded" {
			statusCode = http.StatusOK
		}

		writeJSON(w, statusCode, report)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
