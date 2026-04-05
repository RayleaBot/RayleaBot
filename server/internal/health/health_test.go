package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLivenessHandlerReturnsOKJSON(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	NewLivenessHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var response LivenessResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode liveness response: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("status = %q, want ok", response.Status)
	}
}

func TestNewReadinessHandlerProjectsHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		report     ReadinessReport
		statusCode int
	}{
		{name: "ready", report: ReadinessReport{Status: "ready"}, statusCode: http.StatusOK},
		{name: "degraded", report: ReadinessReport{Status: "degraded"}, statusCode: http.StatusOK},
		{name: "blocked", report: ReadinessReport{Status: "blocked"}, statusCode: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			NewReadinessHandler(func() ReadinessReport { return tt.report }).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if recorder.Code != tt.statusCode {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.statusCode)
			}
		})
	}
}
