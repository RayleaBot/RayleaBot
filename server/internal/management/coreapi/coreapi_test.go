package coreapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
)

func TestSystemStatusIncludesPluginCountsAndDBSchemaVersion(t *testing.T) {
	t.Parallel()

	handlers := NewHandlers(Deps{
		System: coreTestSystem{
			snapshot: systemmodel.StatusSnapshot{
				Status:          "running",
				AdapterState:    "connected",
				ActivePlugins:   2,
				RunningPlugins:  1,
				FailedPlugins:   1,
				DBSchemaVersion: "000004",
				UptimeSeconds:   60,
				Health: &health.ReadinessReport{
					Status: "degraded",
					Checks: map[string]string{"database": "ok", "render": "resource_missing"},
					Issues: []health.DiagnosticIssue{{
						Code:        "render.browser_missing",
						Severity:    "warning",
						Summary:     "浏览器运行资源缺失",
						Remediation: "运行运行时准备任务。",
					}},
				},
			},
		},
	})

	recorder := httptest.NewRecorder()
	handlers.HandleSystemStatus().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/system/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response SystemStatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ActivePlugins != 2 || response.RunningPlugins != 1 || response.FailedPlugins != 1 || response.DBSchemaVersion != "000004" {
		t.Fatalf("unexpected system status response: %#v", response)
	}
	if response.Health == nil || response.Health.Status != "degraded" || response.Health.Checks["render"] != "resource_missing" {
		t.Fatalf("unexpected system health response: %#v", response.Health)
	}
}

func TestIsLoopbackRequestRejectsForwardedHeaders(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/launcher/status", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "127.0.0.1")

	if IsLoopbackRequest(request) {
		t.Fatalf("expected forwarded loopback request to be rejected")
	}
}

type coreTestSystem struct {
	snapshot systemmodel.StatusSnapshot
}

func (s coreTestSystem) StatusSnapshot() systemmodel.StatusSnapshot {
	return s.snapshot
}

func (s coreTestSystem) PublishStatusSnapshot() {}

func TestIsLoopbackRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		want       bool
	}{
		{name: "ipv4 loopback", remoteAddr: "127.0.0.1:12345", want: true},
		{name: "ipv6 loopback", remoteAddr: "[::1]:12345", want: true},
		{name: "localhost", remoteAddr: "localhost:12345", want: true},
		{name: "public host", remoteAddr: "203.0.113.9:12345", want: false},
		{name: "empty", remoteAddr: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "/api/launcher/status", nil)
			request.RemoteAddr = tc.remoteAddr

			if got := IsLoopbackRequest(request); got != tc.want {
				t.Fatalf("IsLoopbackRequest(%q) = %v, want %v", tc.remoteAddr, got, tc.want)
			}
		})
	}
}
