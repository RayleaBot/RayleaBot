package management

import (
	"net/http"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

func NewReadinessHandler(getReport func() systemsvc.ReadinessReport) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		report := getReport()
		statusCode := http.StatusServiceUnavailable
		if report.Status == "ready" || report.Status == "degraded" {
			statusCode = http.StatusOK
		}

		writeJSON(w, statusCode, report)
	}
}

type LivenessResponse struct {
	Status string `json:"status"`
}

func NewLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, LivenessResponse{Status: "ok"})
	}
}
