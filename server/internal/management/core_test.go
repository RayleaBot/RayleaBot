package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
)

func TestSystemStatusIncludesPluginCountsAndDBSchemaVersion(t *testing.T) {
	t.Parallel()

	handlers := NewCoreHandlers(CoreDeps{
		System: coreTestSystem{
			snapshot: systemsvc.StatusSnapshot{
				Status:          "running",
				Adapters:        []adapterservice.Status{{ID: "onebot11", Protocol: "onebot11", Enabled: true, State: "connected"}},
				ActivePlugins:   2,
				RunningPlugins:  1,
				FailedPlugins:   1,
				DBSchemaVersion: "000001",
				UptimeSeconds:   60,
				Health: &systemsvc.ReadinessReport{
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

	var response CoreSystemStatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ActivePlugins != 2 || response.RunningPlugins != 1 || response.FailedPlugins != 1 || response.DBSchemaVersion != "000001" {
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

	if isLoopbackRequest(request) {
		t.Fatalf("expected forwarded loopback request to be rejected")
	}
}

type coreTestSystem struct {
	snapshot systemsvc.StatusSnapshot
}

func (s coreTestSystem) StatusSnapshot() systemsvc.StatusSnapshot {
	return s.snapshot
}

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

			if got := isLoopbackRequest(request); got != tc.want {
				t.Fatalf("isLoopbackRequest(%q) = %v, want %v", tc.remoteAddr, got, tc.want)
			}
		})
	}
}

func TestLauncherShutdownIntentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       systemsvc.StopIntent
	}{
		{"missing body", "", systemsvc.StopIntentStop},
		{"whitespace body", " \n", systemsvc.StopIntentStop},
		{"missing intent", `{}`, systemsvc.StopIntentStop},
		{"stop", `{"intent":"stop"}`, systemsvc.StopIntentStop},
		{"restart", `{"intent":"restart"}`, systemsvc.StopIntentRestart},
		{"update", `{"intent":"update"}`, systemsvc.StopIntentUpdate},
		{"unknown intent", `{"intent":"reboot"}`, ""},
		{"empty intent", `{"intent":""}`, ""},
		{"null intent", `{"intent":null}`, ""},
		{"number intent", `{"intent":1}`, ""},
		{"case sensitive", `{"intent":"Restart"}`, ""},
		{"unknown field", `{"intent":"stop","other":true}`, ""},
		{"trailing JSON", `{} {}`, ""},
		{"non object", `[]`, ""},
		{"null body", `null`, ""},
		{"malformed", `{"intent":`, ""},
		{"oversized", strings.Repeat(" ", 1048577), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got systemsvc.StopIntent
			handlers := NewCoreHandlers(CoreDeps{
				LauncherControlToken: NewStaticToken("fixture-control-token"),
				RequestShutdown:      func(intent systemsvc.StopIntent) { got = intent },
			})
			router := chi.NewRouter()
			handlers.RegisterPublicRoutes(router)
			request := httptest.NewRequest(http.MethodPost, "/api/launcher/shutdown", strings.NewReader(tc.body))
			request.RemoteAddr = "127.0.0.1:12345"
			request.Header.Set(LauncherControlTokenHeader, "fixture-control-token")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if got != tc.want {
				t.Fatalf("shutdown intent = %q, want %q", got, tc.want)
			}
			wantStatus := http.StatusAccepted
			if tc.want == "" {
				wantStatus = http.StatusBadRequest
			}
			if recorder.Code != wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, wantStatus, recorder.Body)
			}
			if tc.want == "" {
				var response struct{ Error struct{ Code string } }
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Error.Code != "platform.invalid_request" {
					t.Fatalf("error response = %s, decode error = %v", recorder.Body, err)
				}
			}
		})
	}
}

func TestShutdownIntentRespectsRouteBoundary(t *testing.T) {
	calls := 0
	handlers := NewCoreHandlers(CoreDeps{
		LauncherControlToken: NewStaticToken("fixture-control-token"),
		RequestShutdown: func(intent systemsvc.StopIntent) {
			calls++
			if intent != systemsvc.StopIntentStop {
				t.Errorf("system shutdown intent = %q", intent)
			}
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/launcher/shutdown", strings.NewReader(`{"intent":"reboot"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	handlers.HandleLauncherShutdown().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("unauthenticated request: status=%d calls=%d", recorder.Code, calls)
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/system/shutdown", strings.NewReader(`{"intent":"restart"}`))
	handlers.HandleSystemShutdown().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || calls != 1 {
		t.Fatalf("system request: status=%d calls=%d", recorder.Code, calls)
	}
}

func TestLauncherStatusReportsCurrentShutdownBudget(t *testing.T) {
	grace := 10
	handlers := NewCoreHandlers(CoreDeps{System: coreTestSystem{snapshot: systemsvc.StatusSnapshot{Status: "running"}}, LauncherControlToken: NewStaticToken("fixture-token"), CurrentConfig: func() config.Config { return config.Config{Runtime: config.RuntimeConfig{ShutdownGraceSeconds: grace}} }})
	for _, value := range []int{10, 60} {
		grace = value
		request := httptest.NewRequest("GET", "/api/launcher/status", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set(LauncherControlTokenHeader, "fixture-token")
		response := httptest.NewRecorder()
		handlers.HandleLauncherStatus().ServeHTTP(response, request)
		var body map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := (config.RuntimeConfig{ShutdownGraceSeconds: grace}).ShutdownBudgets().TotalSeconds()
		if response.Code != 200 || body["shutdown_budget_seconds"] != float64(want) {
			t.Fatalf("status=%d body=%s", response.Code, response.Body)
		}
	}
}
