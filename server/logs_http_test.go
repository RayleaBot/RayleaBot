package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rayleabot/server/internal/logging"
)

func TestLogsListReturnsFilteredSummaries(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.yaml"))

	application.Logs.Append(logging.Summary{
		Timestamp: "2026-03-20T09:59:59Z",
		Level:     "warn",
		Source:    "runtime",
		Message:   "ignored warning",
	})
	application.Logs.Append(logging.Summary{
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "error",
		Source:    "runtime",
		Message:   "plugin runtime stderr truncated",
		PluginID:  "weather",
		RequestID: "req_plugin_0001",
	})
	application.Logs.Append(logging.Summary{
		Timestamp: "2026-03-20T10:00:01Z",
		Level:     "error",
		Source:    "adapter.onebot11",
		Message:   "reverse websocket connection lost",
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create logs list request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform logs list request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected logs list status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected logs list body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListReturnsEmptyArrayForUnmatchedFilter(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.logs-empty-response.yaml"))
	application.Logs.Append(logging.Summary{
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "info",
		Source:    "adapter.onebot11",
		Message:   "connected",
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create empty logs request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform empty logs request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected empty logs status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected empty logs body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs?level=fatal&limit=999", nil)
	if err != nil {
		t.Fatalf("create invalid logs request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform invalid logs request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected invalid logs status: got %d want 400", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
}

func TestLogsListDoesNotLeakRawAttrs(t *testing.T) {
	t.Parallel()

	application := newTestAppWithOneBotAccessToken(t, "fixture-only-secret", deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	application.Logger.Error(
		"downstream rejected fixture-only-secret during adapter handshake",
		"component", "runtime",
		"plugin_id", "weather",
		"request_id", "req_log_0001",
		"secret", "fixture-only-secret",
		"token", "session-token-abc",
	)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs?limit=1", nil)
	if err != nil {
		t.Fatalf("create logs redaction request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform logs redaction request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected logs redaction status: got %d want 200", response.StatusCode)
	}

	raw := string(readAll(t, response))
	if strings.Contains(raw, "fixture-only-secret") || strings.Contains(raw, "session-token-abc") {
		t.Fatalf("logs response leaked sensitive content: %s", raw)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode logs body: %v", err)
	}
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected logs items length: %#v", body["items"])
	}
	item := items[0].(map[string]any)
	allowed := map[string]bool{
		"timestamp":  true,
		"level":      true,
		"source":     true,
		"message":    true,
		"plugin_id":  true,
		"request_id": true,
	}
	for key := range item {
		if !allowed[key] {
			t.Fatalf("unexpected logs field %q", key)
		}
	}
	if !strings.Contains(item["message"].(string), "[REDACTED]") {
		t.Fatalf("expected redacted message, got %#v", item["message"])
	}
}

func TestLogsRouteRequiresAuth(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs", nil)
	if err != nil {
		t.Fatalf("create logs auth request: %v", err)
	}

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform logs auth request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected logs auth status: got %d want 401", response.StatusCode)
	}
}
