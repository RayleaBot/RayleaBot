package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
	"rayleabot/server/internal/adapter"

	"rayleabot/server/internal/app"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/health"
)

type webAPIFixture struct {
	Response struct {
		Status int            `yaml:"status"`
		Body   map[string]any `yaml:"body"`
	} `yaml:"response"`
}

func TestHealthzResponseMatchesFixture(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "ok.healthz-response.yaml"))

	request := httptest.NewRequest("GET", "/healthz", nil)
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal /healthz body: %v", err)
	}

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected /healthz body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestReadyzConnectedStateIsReady(t *testing.T) {
	t.Parallel()

	assertReadinessResponse(
		t,
		adapter.Snapshot{
			State:                 adapter.StateConnected,
			TotalReceivedFrames:   7,
			InvalidReceivedFrames: 2,
			HeartbeatSeen:         true,
			LastFrameCategory:     adapter.FrameCategoryInvalid,
			LastFrameType:         "invalid",
		},
		http.StatusOK,
		map[string]any{
			"status": "ready",
			"checks": map[string]any{
				"config":   "ok",
				"database": "ok",
				"runtime":  "ok",
				"adapter":  "ok",
				"render":   "ok",
			},
		},
	)
}

func TestReadyzAuthFailedIsDegraded(t *testing.T) {
	t.Parallel()

	assertReadinessResponse(
		t,
		adapter.Snapshot{
			State:         adapter.StateAuthFailed,
			LastErrorCode: "adapter.auth_failed",
		},
		http.StatusOK,
		map[string]any{
			"status": "degraded",
			"reason": "OneBot authentication failed",
			"reason_codes": []any{
				"adapter.auth_failed",
			},
			"checks": map[string]any{
				"config":   "ok",
				"database": "ok",
				"runtime":  "ok",
				"adapter":  "auth_failed",
				"render":   "ok",
			},
			"issues": []any{
				map[string]any{
					"code":        "adapter.auth_failed",
					"severity":    "warning",
					"summary":     "OneBot authentication failed",
					"remediation": "请检查 OneBot access_token 配置后重试连接。",
				},
			},
		},
	)
}

func TestReadyzReconnectingIsDegraded(t *testing.T) {
	t.Parallel()

	assertReadinessResponse(
		t,
		adapter.Snapshot{
			State:         adapter.StateReconnecting,
			LastErrorCode: "adapter.connection_lost",
		},
		http.StatusOK,
		map[string]any{
			"status": "degraded",
			"reason": "OneBot reverse WebSocket is reconnecting",
			"reason_codes": []any{
				"adapter.connection_lost",
			},
			"checks": map[string]any{
				"config":   "ok",
				"database": "ok",
				"runtime":  "ok",
				"adapter":  "reconnecting",
				"render":   "ok",
			},
			"issues": []any{
				map[string]any{
					"code":        "adapter.connection_lost",
					"severity":    "warning",
					"summary":     "OneBot reverse WebSocket is reconnecting",
					"remediation": "请检查 OneBot 服务可用性，或等待连接自动恢复。",
				},
			},
		},
	)
}

func TestReadinessHandlerEncodesDegradedFixtureShape(t *testing.T) {
	t.Parallel()

	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "edge.readyz-degraded-response.yaml"))
	checks := map[string]string{}
	for key, value := range fixture.Response.Body["checks"].(map[string]any) {
		checks[key] = value.(string)
	}

	var issues []health.DiagnosticIssue
	if rawIssues, ok := fixture.Response.Body["issues"].([]any); ok {
		for _, raw := range rawIssues {
			m := raw.(map[string]any)
			issue := health.DiagnosticIssue{
				Code:     m["code"].(string),
				Severity: m["severity"].(string),
				Summary:  m["summary"].(string),
			}
			if rem, ok := m["remediation"].(string); ok {
				issue.Remediation = rem
			}
			issues = append(issues, issue)
		}
	}

	report := health.ReadinessReport{
		Status:      fixture.Response.Body["status"].(string),
		Reason:      fixture.Response.Body["reason"].(string),
		ReasonCodes: toStringSlice(fixture.Response.Body["reason_codes"].([]any)),
		Checks:      checks,
		Issues:      issues,
	}

	handler := health.NewReadinessHandler(func() health.ReadinessReport {
		return report
	})

	request := httptest.NewRequest("GET", "/readyz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected degraded status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal degraded body: %v", err)
	}

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected degraded body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestReadyzReportsSetupRequiredBeforeBootstrap(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "edge.readyz-setup-required-response.yaml"))
	request := httptest.NewRequest("GET", "/readyz", nil)
	recorder := httptest.NewRecorder()

	application.Handler().ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected setup-required status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal setup-required body: %v", err)
	}

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected setup-required body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func newTestApp(t *testing.T, authOptions ...auth.Option) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))
	configPath := writeYAMLConfig(t, fixture.Input)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := app.New(app.Options{
		ConfigPath:  configPath,
		SchemaPath:  schemaPath,
		AuthOptions: authOptions,
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application
}

func loadWebAPIFixture(t *testing.T, path string) webAPIFixture {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture webAPIFixture
	if err := yaml.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}

	return fixture
}

func toStringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.(string))
	}

	return result
}

func assertReadinessResponse(t *testing.T, snapshot adapter.Snapshot, wantStatus int, wantBody map[string]any) {
	t.Helper()

	handler := health.NewReadinessHandler(func() health.ReadinessReport {
		return app.ReadinessReportFromAdapter(snapshot)
	})

	request := httptest.NewRequest("GET", "/readyz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != wantStatus {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, wantStatus)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal /readyz body: %v", err)
	}

	if !reflect.DeepEqual(body, wantBody) {
		t.Fatalf("unexpected /readyz body: got %#v want %#v", body, wantBody)
	}
}
