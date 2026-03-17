package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"rayleabot/server/internal/adapter"
	"gopkg.in/yaml.v3"

	"rayleabot/server/internal/app"
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
		adapter.Snapshot{State: adapter.StateConnected},
		http.StatusOK,
		map[string]any{
			"status": "ready",
			"checks": map[string]any{
				"config":  "ok",
				"adapter": "connected",
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
				"config":  "ok",
				"adapter": "auth_failed",
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
				"config":  "ok",
				"adapter": "reconnecting",
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

	report := health.ReadinessReport{
		Status:      fixture.Response.Body["status"].(string),
		Reason:      fixture.Response.Body["reason"].(string),
		ReasonCodes: toStringSlice(fixture.Response.Body["reason_codes"].([]any)),
		Checks:      checks,
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

func newTestApp(t *testing.T) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))
	configPath := writeYAMLConfig(t, fixture.Input)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := app.New(app.Options{
		ConfigPath: configPath,
		SchemaPath: schemaPath,
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}

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
