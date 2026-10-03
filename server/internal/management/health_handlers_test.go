package management

import (
	"net/http"
	"net/http/httptest"
	"testing"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
)

func TestNewReadinessHandlerProjectsHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		report     systemsvc.ReadinessReport
		statusCode int
	}{
		{name: "ready", report: systemsvc.ReadinessReport{Status: "ready"}, statusCode: http.StatusOK},
		{name: "degraded", report: systemsvc.ReadinessReport{Status: "degraded"}, statusCode: http.StatusOK},
		{name: "blocked", report: systemsvc.ReadinessReport{Status: "blocked"}, statusCode: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			NewReadinessHandler(func() systemsvc.ReadinessReport { return tt.report }).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if recorder.Code != tt.statusCode {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.statusCode)
			}
		})
	}
}
