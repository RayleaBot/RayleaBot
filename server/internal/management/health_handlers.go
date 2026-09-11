package management

import (
	"net/http"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
)

func NewReadinessHandler(getReport func() systemsvc.ReadinessReport) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		report := getReport()
		statusCode := http.StatusServiceUnavailable
		if report.Status == "ready" || report.Status == "degraded" {
			statusCode = http.StatusOK
		}

		httpapi.WriteJSON(w, statusCode, report)
	}
}

type livenessResponse struct {
	Status string `json:"status"`
}

func NewLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, livenessResponse{Status: "ok"})
	}
}
