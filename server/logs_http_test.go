package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func TestLogsListReturnsFilteredSummaries(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.yaml"))

	application.Logs().Append(logging.Summary{
		LogID:     "log_warn_0001",
		Timestamp: "2026-03-20T09:59:59Z",
		Level:     "warn",
		Source:    "runtime",
		Message:   "ignored warning",
	})
	application.Logs().Append(logging.Summary{
		LogID:     "log_runtime_0001",
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "error",
		Source:    "runtime",
		Message:   "plugin runtime stderr truncated",
		PluginID:  "weather",
		RequestID: "req_plugin_0001",
	})
	application.Logs().Append(logging.Summary{
		LogID:     "log_adapter_0001",
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

func TestLogsListReturnsProtocolFilteredSummaries(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.protocol-onebot11.yaml"))

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_protocol_0001",
			Timestamp: "2026-03-20T10:00:00Z",
			Level:     "warn",
			Source:    "adapter",
			Message:   "adapter reconnect scheduled",
		},
		{
			LogID:     "log_protocol_0002",
			Timestamp: "2026-03-20T10:00:01Z",
			Level:     "error",
			Source:    "adapter.onebot11",
			Message:   "reverse websocket authentication failed",
			RequestID: "req_adapter_0002",
		},
		{
			LogID:     "log_protocol_0003",
			Timestamp: "2026-03-20T10:00:02Z",
			Level:     "info",
			Source:    "bridge",
			Message:   "runtime bridge delivered adapter event",
			RequestID: "req_bridge_0001",
		},
		{
			Timestamp: "2026-03-20T10:00:03Z",
			Level:     "warn",
			Source:    "runtime",
			Message:   "plugin runtime stderr truncated",
		},
	} {
		application.Logs().Append(summary)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create protocol logs list request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform protocol logs list request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected protocol logs status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected protocol logs body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListReturnsEmptyArrayForUnmatchedFilter(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.logs-empty-response.yaml"))
	application.Logs().Append(logging.Summary{
		LogID:     "log_empty_0001",
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

func TestLogsListReturnsEmptyArrayForUnmatchedProtocolFilter(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.logs-empty-response.protocol-onebot11.yaml"))
	application.Logs().Append(logging.Summary{
		LogID:     "log_runtime_0002",
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "runtime only",
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create empty protocol logs request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform empty protocol logs request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected empty protocol logs status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected empty protocol logs body: got %#v want %#v", body, fixture.Response.Body)
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

	application.Logger().Error(
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
		"log_id":     true,
		"timestamp":  true,
		"level":      true,
		"source":     true,
		"message":    true,
		"protocol":   true,
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

func TestLogDetailReturnsStructuredDetails(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.log-detail-response.yaml"))

	application.Logs().Append(logging.Summary{
		LogID:     "log_bridge_0001",
		Timestamp: "2026-03-20T10:00:02Z",
		Level:     "info",
		Source:    "bridge",
		Message:   "runtime bridge delivered adapter event",
		RequestID: "req_bridge_0001",
		Details: map[string]any{
			"direction":         "inbound",
			"event_kind":        "onebot11.message",
			"event_type":        "message.group",
			"post_type":         "message",
			"message_type":      "group",
			"event_timestamp":   float64(1711015202),
			"time":              float64(1711015202),
			"conversation_type": "group",
			"conversation_id":   "2001",
			"group_id":          "2001",
			"sender_id":         "3001",
			"user_id":           "3001",
			"sender_nickname":   "Alice",
			"sender_card":       "管理员",
			"sender_role":       "admin",
			"message_id":        "1001",
			"real_id":           "1001",
			"message_seq":       "1001",
			"raw_message":       "hello bridge",
			"message_format":    "array",
			"font":              float64(14),
			"plain_text":        "hello bridge",
			"sender": map[string]any{
				"user_id":  "3001",
				"nickname": "Alice",
				"card":     "管理员",
				"role":     "admin",
			},
			"segments": []any{
				map[string]any{
					"type": "text",
					"data": map[string]any{"text": "hello bridge"},
				},
			},
		},
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create log detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform log detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected log detail status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected log detail body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogDetailReturnsEmptyObjectForLegacyRows(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.log-detail-legacy-empty-details.yaml"))

	application.Logs().Append(logging.Summary{
		LogID:     "log_legacy_0001",
		Timestamp: "2026-03-20T10:00:01Z",
		Level:     "warn",
		Source:    "adapter",
		Message:   "adapter reconnect scheduled",
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create legacy log detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform legacy log detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected legacy log detail status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected legacy log detail body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogDetailReturnsNotFound(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.log-detail-not-found.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create missing log detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform missing log detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected missing log detail status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.resource_missing")
}

func TestLogDetailFallsBackToLiveStreamWhenRepositoryMissesNewLog(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)

	application.SetLogRepository(&stubMissingLogRepository{})
	application.Logs().Append(logging.Summary{
		LogID:     "log_live_only_0001",
		Timestamp: "2026-04-09T20:51:46Z",
		Level:     "info",
		Source:    "bridge",
		Message:   "runtime bridge delivered group message: 装修臭头大",
		RequestID: "dispatch_1775739204056693800",
		Details: map[string]any{
			"direction":       "inbound",
			"event_type":      "message.group",
			"conversation_id": "860105388",
			"group_id":        "860105388",
			"sender_nickname": "Alice",
			"plain_text":      "装修臭头大",
		},
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs/log_live_only_0001", nil)
	if err != nil {
		t.Fatalf("create live stream fallback request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform live stream fallback request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected live stream fallback status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["log_id"] != "log_live_only_0001" {
		t.Fatalf("unexpected fallback log id: %#v", body["log_id"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected fallback details map, got %#v", body["details"])
	}
	if details["plain_text"] != "装修臭头大" {
		t.Fatalf("unexpected fallback details: %#v", details)
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

type stubMissingLogRepository struct{}

func (*stubMissingLogRepository) SaveSummary(context.Context, logging.Summary) error {
	return nil
}

func (*stubMissingLogRepository) ListSummaries(context.Context, logging.Query) ([]logging.Summary, error) {
	return nil, nil
}

func (*stubMissingLogRepository) GetSummary(context.Context, string) (logging.Summary, error) {
	return logging.Summary{}, logging.ErrLogNotFound
}

func (*stubMissingLogRepository) PruneOlderThan(context.Context, time.Time) error {
	return nil
}

func TestLogsListReadsPersistedSummariesAcrossRestart(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	appA := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC) }, "logs-a")
	tokenA := issueLoginToken(t, appA)
	serverA := httptest.NewServer(appA.Handler())

	requestA, err := http.NewRequest(http.MethodGet, serverA.URL+"/api/logs?limit=1", nil)
	if err != nil {
		t.Fatalf("create seed request: %v", err)
	}
	requestA.Header.Set("Authorization", "Bearer "+tokenA)
	responseA, err := serverA.Client().Do(requestA)
	if err != nil {
		t.Fatalf("perform seed request: %v", err)
	}
	responseA.Body.Close()

	appA.Logger().Error(
		"persisted log survives restart",
		"component", "runtime",
		"plugin_id", "weather",
		"request_id", "req_persist_1",
	)

	serverA.Close()
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 5, 0, 0, time.UTC) }, "logs-b")
	defer closePersistentTestApp(t, appB)
	tokenB := issueExistingBootstrapLoginToken(t, appB)
	serverB := httptest.NewServer(appB.Handler())
	defer serverB.Close()

	requestB, err := http.NewRequest(http.MethodGet, serverB.URL+"/api/logs?request_id=req_persist_1&limit=10", nil)
	if err != nil {
		t.Fatalf("create persisted logs request: %v", err)
	}
	requestB.Header.Set("Authorization", "Bearer "+tokenB)

	responseB, err := serverB.Client().Do(requestB)
	if err != nil {
		t.Fatalf("perform persisted logs request: %v", err)
	}
	defer responseB.Body.Close()
	if responseB.StatusCode != http.StatusOK {
		t.Fatalf("unexpected persisted logs status: got %d want 200", responseB.StatusCode)
	}

	body := decodeBody(t, readAll(t, responseB))
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected persisted logs count: %#v", body["items"])
	}
	item := items[0].(map[string]any)
	if item["message"] != "persisted log survives restart" {
		t.Fatalf("unexpected persisted log message: %#v", item["message"])
	}
	if item["log_id"] == "" {
		t.Fatalf("expected persisted log_id, got %#v", item["log_id"])
	}
	if item["plugin_id"] != "weather" || item["request_id"] != "req_persist_1" {
		t.Fatalf("unexpected persisted log envelope: %#v", item)
	}
}
