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

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
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
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected logs list body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListReturnsMultiFilteredSummaries(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.multi-filter.yaml"))

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_multi_filter_0001",
			Timestamp: "2026-03-20T10:00:00Z",
			Level:     "error",
			Source:    "runtime",
			Message:   "weather plugin error",
			PluginID:  "weather",
			RequestID: "req_weather_0001",
		},
		{
			LogID:     "log_multi_filter_0002",
			Timestamp: "2026-03-20T10:00:01Z",
			Level:     "warn",
			Source:    "runtime",
			Message:   "help plugin warning",
			PluginID:  "help",
			RequestID: "req_help_0001",
		},
		{
			LogID:     "log_multi_filter_0003",
			Timestamp: "2026-03-20T10:00:02Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "filtered by level",
			PluginID:  "weather",
		},
		{
			LogID:     "log_multi_filter_0004",
			Timestamp: "2026-03-20T10:00:03Z",
			Level:     "error",
			Source:    "runtime",
			Message:   "filtered by plugin",
			PluginID:  "ops",
		},
	} {
		application.Logs().Append(summary)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create logs multi-filter request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform logs multi-filter request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected logs multi-filter status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected logs multi-filter body: got %#v want %#v", body, fixture.Response.Body)
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
			Message:   "721011692: [测试群(2001)]管理员/Alice(3001): hello bridge",
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
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected protocol logs body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListReturnsOutboundProtocolFilteredSummaries(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.protocol-onebot11.outbound-message.yaml"))

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_outbound_delivered_0001",
			Timestamp: "2026-04-10T09:18:00Z",
			Level:     "info",
			Source:    "adapter.onebot11",
			Message:   "weather/echo -> [测试群(2001)]：hello world",
			PluginID:  "weather",
			RequestID: "req_runtime_delivery_0001",
		},
		{
			LogID:     "log_outbound_failed_0001",
			Timestamp: "2026-04-10T09:18:01Z",
			Level:     "warn",
			Source:    "adapter.onebot11",
			Message:   "help/help -> Alice(3001) 发送失败：hello world",
			PluginID:  "help",
			RequestID: "req_runtime_delivery_0002",
		},
		{
			LogID:     "log_runtime_ignored_0001",
			Timestamp: "2026-04-10T09:18:02Z",
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
		t.Fatalf("create outbound protocol logs request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform outbound protocol logs request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected outbound protocol logs status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected outbound protocol logs body: got %#v want %#v", body, fixture.Response.Body)
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
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
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
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected empty protocol logs body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs?level=warn&level=fatal&limit=50", nil)
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

func TestLogsListRejectsLimitAboveFormalMaximum(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.logs-list-limit-too-large.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create large limit request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform large limit request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected large limit status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, readAll(t, response)), fixture.Response.Body, "platform.invalid_request")
}

func TestLogsListReturnsCurrentSessionScope(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_current_session_0001",
			Timestamp: "2026-03-20T10:00:00Z",
			Level:     "warn",
			Source:    "adapter.onebot11",
			Message:   "reverse websocket connection lost",
			RequestID: "req_current_scope",
		},
		{
			LogID:     "log_current_session_0002",
			Timestamp: "2026-03-20T10:00:01Z",
			Level:     "error",
			Source:    "runtime",
			Message:   "plugin runtime stderr truncated",
			PluginID:  "weather",
			RequestID: "req_current_scope",
		},
		{
			LogID:     "log_current_session_0003",
			Timestamp: "2026-03-20T10:00:02Z",
			Level:     "info",
			Source:    "bridge",
			Message:   "721011692: [测试群(2001)]管理员/Alice(3001): hello bridge",
			RequestID: "req_current_scope",
		},
	} {
		application.Logs().Append(summary)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs?scope=current_session&request_id=req_current_scope&limit=3", nil)
	if err != nil {
		t.Fatalf("create current session logs request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform current session logs request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected current session logs status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	items := body["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("unexpected current session logs count: %#v", items)
	}
	if items[0].(map[string]any)["message"] != "721011692: [测试群(2001)]管理员/Alice(3001): hello bridge" {
		t.Fatalf("unexpected first current session item: %#v", items[0])
	}
	if items[1].(map[string]any)["message"] != "plugin runtime stderr truncated" {
		t.Fatalf("unexpected second current session item: %#v", items[1])
	}
	if items[2].(map[string]any)["message"] != "reverse websocket connection lost" {
		t.Fatalf("unexpected third current session item: %#v", items[2])
	}

	page := body["page"].(map[string]any)
	if page["has_older"] != false || page["has_newer"] != false {
		t.Fatalf("unexpected current session page info: %#v", page)
	}
}

func TestLogsListReturnsHistoryRange(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.logs-list-response.history-range.yaml"))

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_history_range_ignored_0001",
			Timestamp: "2026-03-19T23:59:59Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "too early",
		},
		{
			LogID:     "log_history_range_0001",
			Timestamp: "2026-03-20T00:05:00Z",
			Level:     "warn",
			Source:    "adapter.onebot11",
			Message:   "reverse websocket authentication failed",
			RequestID: "req_adapter_0002",
		},
		{
			LogID:     "log_history_range_0002",
			Timestamp: "2026-03-20T10:00:01Z",
			Level:     "error",
			Source:    "runtime",
			Message:   "plugin runtime stderr truncated",
			PluginID:  "weather",
			RequestID: "req_plugin_0001",
		},
		{
			LogID:     "log_history_range_0003",
			Timestamp: "2026-03-20T20:12:00Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "recovery summary refreshed",
		},
		{
			LogID:     "log_history_range_ignored_0002",
			Timestamp: "2026-03-21T00:00:00Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "too late",
		},
	} {
		application.Logs().Append(summary)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create history range request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform history range request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected history range status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, normalizeJSONMap(t, fixture.Response.Body)) {
		t.Fatalf("unexpected history range body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsListReturnsHistoryRangeForOffsetTimestamps(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)

	for _, summary := range []logging.Summary{
		{
			LogID:     "log_history_offset_0001",
			Timestamp: "2026-04-17T02:02:41+08:00",
			Level:     "info",
			Source:    "runtime",
			Message:   "offset row stays visible in history",
		},
		{
			LogID:     "log_history_offset_0002",
			Timestamp: "2026-04-17T02:05:01+08:00",
			Level:     "info",
			Source:    "runtime",
			Message:   "outside offset range",
		},
	} {
		application.Logs().Append(summary)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs?scope=history&start_at=2026-04-16T18:00:00Z&end_at=2026-04-16T18:04:00Z&limit=10", nil)
	if err != nil {
		t.Fatalf("create offset history range request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform offset history range request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected offset history range status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected offset history item count: %#v", items)
	}
	if items[0].(map[string]any)["message"] != "offset row stays visible in history" {
		t.Fatalf("unexpected offset history item: %#v", items[0])
	}
}

func TestLogsListRejectsInvalidScope(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.logs-list-invalid-scope.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create invalid scope request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform invalid scope request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected invalid scope status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, readAll(t, response)), fixture.Response.Body, "platform.invalid_request")
}

func TestLogsListRejectsStartAfterEnd(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.logs-list-start-after-end.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create start-after-end request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform start-after-end request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected start-after-end status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, readAll(t, response)), fixture.Response.Body, "platform.invalid_request")
}

func TestLogsListRejectsCurrentSessionTimeRange(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.logs-list-current-session-with-time-range.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create current-session-with-time-range request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform current-session-with-time-range request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected current-session-with-time-range status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, readAll(t, response)), fixture.Response.Body, "platform.invalid_request")
}

func TestLogsListSupportsCursorPaging(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	for _, summary := range []logging.Summary{
		{LogID: "log_cursor_0001", Timestamp: "2026-04-10T09:00:00Z", Level: "info", Source: "runtime", Message: "1"},
		{LogID: "log_cursor_0002", Timestamp: "2026-04-10T09:00:01Z", Level: "info", Source: "runtime", Message: "2"},
		{LogID: "log_cursor_0003", Timestamp: "2026-04-10T09:00:02Z", Level: "info", Source: "runtime", Message: "3"},
		{LogID: "log_cursor_0004", Timestamp: "2026-04-10T09:00:03Z", Level: "info", Source: "runtime", Message: "4"},
		{LogID: "log_cursor_0005", Timestamp: "2026-04-10T09:00:04Z", Level: "info", Source: "runtime", Message: "5"},
	} {
		application.Logs().Append(summary)
	}

	firstPage := doLogsListRequest(t, server.URL, token, "/api/logs?source=runtime&limit=2")
	firstItems := firstPage["items"].([]any)
	if firstItems[0].(map[string]any)["message"] != "5" || firstItems[1].(map[string]any)["message"] != "4" {
		t.Fatalf("unexpected first page items: %#v", firstItems)
	}

	firstPageInfo := firstPage["page"].(map[string]any)
	olderCursor, ok := firstPageInfo["older_cursor"].(string)
	if !ok || olderCursor == "" {
		t.Fatalf("expected older cursor on first page: %#v", firstPageInfo)
	}
	if firstPageInfo["has_newer"] != false || firstPageInfo["has_older"] != true {
		t.Fatalf("unexpected first page metadata: %#v", firstPageInfo)
	}

	secondPage := doLogsListRequest(t, server.URL, token, "/api/logs?source=runtime&limit=2&direction=older&cursor="+olderCursor)
	secondItems := secondPage["items"].([]any)
	if secondItems[0].(map[string]any)["message"] != "3" || secondItems[1].(map[string]any)["message"] != "2" {
		t.Fatalf("unexpected second page items: %#v", secondItems)
	}

	secondPageInfo := secondPage["page"].(map[string]any)
	newerCursor, ok := secondPageInfo["newer_cursor"].(string)
	if !ok || newerCursor == "" {
		t.Fatalf("expected newer cursor on second page: %#v", secondPageInfo)
	}
	if secondPageInfo["has_newer"] != true || secondPageInfo["has_older"] != true {
		t.Fatalf("unexpected second page metadata: %#v", secondPageInfo)
	}

	thirdPage := doLogsListRequest(t, server.URL, token, "/api/logs?source=runtime&limit=2&direction=newer&cursor="+newerCursor)
	thirdItems := thirdPage["items"].([]any)
	if thirdItems[0].(map[string]any)["message"] != "5" || thirdItems[1].(map[string]any)["message"] != "4" {
		t.Fatalf("unexpected newer page items: %#v", thirdItems)
	}
}

func TestLogsListSupportsCursorPagingWithMultiFilters(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	for _, summary := range []logging.Summary{
		{LogID: "log_multi_cursor_0001", Timestamp: "2026-04-10T09:00:00Z", Level: "info", Source: "runtime", Message: "1", PluginID: "weather"},
		{LogID: "log_multi_cursor_0002", Timestamp: "2026-04-10T09:00:01Z", Level: "warn", Source: "runtime", Message: "filtered by level", PluginID: "help"},
		{LogID: "log_multi_cursor_0003", Timestamp: "2026-04-10T09:00:02Z", Level: "error", Source: "runtime", Message: "2", PluginID: "help"},
		{LogID: "log_multi_cursor_0004", Timestamp: "2026-04-10T09:00:03Z", Level: "info", Source: "runtime", Message: "filtered by plugin", PluginID: "ops"},
		{LogID: "log_multi_cursor_0005", Timestamp: "2026-04-10T09:00:04Z", Level: "error", Source: "runtime", Message: "3", PluginID: "weather"},
		{LogID: "log_multi_cursor_0006", Timestamp: "2026-04-10T09:00:05Z", Level: "info", Source: "runtime", Message: "4", PluginID: "help"},
	} {
		application.Logs().Append(summary)
	}

	filterPath := "/api/logs?source=runtime&level=info&level=error&plugin_id=weather&plugin_id=help&limit=2"
	firstPage := doLogsListRequest(t, server.URL, token, filterPath)
	firstItems := firstPage["items"].([]any)
	if firstItems[0].(map[string]any)["message"] != "4" || firstItems[1].(map[string]any)["message"] != "3" {
		t.Fatalf("unexpected multi-filter first page items: %#v", firstItems)
	}

	firstPageInfo := firstPage["page"].(map[string]any)
	olderCursor, ok := firstPageInfo["older_cursor"].(string)
	if !ok || olderCursor == "" {
		t.Fatalf("expected older cursor on multi-filter first page: %#v", firstPageInfo)
	}
	if firstPageInfo["has_newer"] != false || firstPageInfo["has_older"] != true {
		t.Fatalf("unexpected multi-filter first page metadata: %#v", firstPageInfo)
	}

	secondPage := doLogsListRequest(t, server.URL, token, filterPath+"&direction=older&cursor="+olderCursor)
	secondItems := secondPage["items"].([]any)
	if secondItems[0].(map[string]any)["message"] != "2" || secondItems[1].(map[string]any)["message"] != "1" {
		t.Fatalf("unexpected multi-filter second page items: %#v", secondItems)
	}

	secondPageInfo := secondPage["page"].(map[string]any)
	if secondPageInfo["has_newer"] != true || secondPageInfo["has_older"] != false {
		t.Fatalf("unexpected multi-filter second page metadata: %#v", secondPageInfo)
	}
}

func TestLogsListIgnoresNewerDirectionWithoutCursorOnFirstPage(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	for _, summary := range []logging.Summary{
		{LogID: "log_cursor_1001", Timestamp: "2026-04-10T09:00:00Z", Level: "info", Source: "runtime", Message: "1"},
		{LogID: "log_cursor_1002", Timestamp: "2026-04-10T09:00:01Z", Level: "info", Source: "runtime", Message: "2"},
		{LogID: "log_cursor_1003", Timestamp: "2026-04-10T09:00:02Z", Level: "info", Source: "runtime", Message: "3"},
		{LogID: "log_cursor_1004", Timestamp: "2026-04-10T09:00:03Z", Level: "info", Source: "runtime", Message: "4"},
		{LogID: "log_cursor_1005", Timestamp: "2026-04-10T09:00:04Z", Level: "info", Source: "runtime", Message: "5"},
	} {
		application.Logs().Append(summary)
	}

	defaultPage := doLogsListRequest(t, server.URL, token, "/api/logs?source=runtime&limit=2")
	newerFirstPage := doLogsListRequest(t, server.URL, token, "/api/logs?source=runtime&limit=2&direction=newer")

	if !reflect.DeepEqual(newerFirstPage, defaultPage) {
		t.Fatalf("unexpected first page for direction=newer without cursor: got %#v want %#v", newerFirstPage, defaultPage)
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
		Message:   "721011692: [测试群(2001)]管理员/Alice(3001): hello bridge",
		RequestID: "req_bridge_0001",
		Details: map[string]any{
			"direction":         "inbound",
			"event_kind":        "onebot11.message",
			"event_type":        "message.group",
			"post_type":         "message",
			"message_type":      "group",
			"event_timestamp":   float64(1711015202),
			"self_id":           "721011692",
			"time":              float64(1711015202),
			"conversation_type": "group",
			"conversation_id":   "2001",
			"group_name":        "测试群",
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

func TestLogDetailReturnsOutboundStructuredDetail(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.log-detail-response.outbound-onebot11.yaml"))

	application.Logs().Append(logging.Summary{
		LogID:     "log_outbound_delivered_0001",
		Timestamp: "2026-04-10T09:18:00Z",
		Level:     "info",
		Source:    "adapter.onebot11",
		Message:   "weather/echo -> [测试群(2001)]：hello world",
		PluginID:  "weather",
		RequestID: "req_runtime_delivery_0001",
		Details: map[string]any{
			"direction":     "outbound",
			"action_kind":   "message.send",
			"delivery_kind": "message.send",
			"command_name":  "echo",
			"target_type":   "group",
			"target_id":     "2001",
			"plain_text":    "hello world",
			"message_id":    "966671988",
			"segments": []any{
				map[string]any{
					"type": "text",
					"data": map[string]any{"text": "hello world"},
				},
			},
		},
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create outbound log detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform outbound log detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected outbound log detail status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected outbound log detail body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestLogsIncludeCommandPolicyRejectionFromEventIngress(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	putWhitelistState(t, server.URL, token, true)
	application.HandleAdapterEvent(context.Background(), commandRejectionEvent())

	listBody := doLogsListRequest(t, server.URL, token, "/api/logs?protocol=onebot11&limit=20")
	items := listBody["items"].([]any)

	var rejectionSummary map[string]any
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["message"] == "plugin raylea.help command help rejected by command policy: sender is not whitelisted" {
			rejectionSummary = item
			break
		}
	}
	if rejectionSummary == nil {
		t.Fatalf("expected command policy rejection in log list, got %#v", items)
	}
	if rejectionSummary["source"] != "bridge" || rejectionSummary["protocol"] != "onebot11" {
		t.Fatalf("unexpected command rejection summary: %#v", rejectionSummary)
	}
	if rejectionSummary["plugin_id"] != "raylea.help" {
		t.Fatalf("unexpected command rejection plugin_id: %#v", rejectionSummary["plugin_id"])
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs/"+rejectionSummary["log_id"].(string), nil)
	if err != nil {
		t.Fatalf("create command rejection detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform command rejection detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected command rejection detail status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["plugin_id"] != "raylea.help" {
		t.Fatalf("unexpected command rejection detail plugin_id: %#v", body["plugin_id"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected command rejection details payload: %#v", body["details"])
	}
	if details["command_name"] != "help" || details["error_code"] != "permission.not_whitelisted" {
		t.Fatalf("unexpected command rejection details: %#v", details)
	}
	if details["reason"] != "actor is not whitelisted" || details["policy_stage"] != "whitelist" {
		t.Fatalf("unexpected command rejection details: %#v", details)
	}
	if !reflect.DeepEqual(details["matched_plugin_ids"], []any{"raylea.help"}) {
		t.Fatalf("unexpected matched_plugin_ids detail: %#v", details["matched_plugin_ids"])
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
		Message:   "721011692: [测试群(860105388)]Alice(3001): 装修臭头大",
		RequestID: "dispatch_1775739204056693800",
		Details: map[string]any{
			"direction":       "inbound",
			"event_type":      "message.group",
			"self_id":         "721011692",
			"conversation_id": "860105388",
			"group_name":      "测试群",
			"group_id":        "860105388",
			"sender_id":       "3001",
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
	if details["self_id"] != "721011692" {
		t.Fatalf("unexpected self_id detail: %#v", details["self_id"])
	}
	if details["group_name"] != "测试群" {
		t.Fatalf("unexpected group_name detail: %#v", details["group_name"])
	}
	if _, ok := details["group_id"]; ok {
		t.Fatalf("group_id should be omitted from compacted fallback detail: %#v", details)
	}
	if _, ok := details["sender_nickname"]; ok {
		t.Fatalf("sender_nickname should be omitted from compacted fallback detail: %#v", details)
	}
	sender, ok := details["sender"].(map[string]any)
	if !ok {
		t.Fatalf("expected compacted sender map, got %#v", details["sender"])
	}
	if sender["user_id"] != "3001" || sender["nickname"] != "Alice" {
		t.Fatalf("unexpected compacted sender details: %#v", sender)
	}
}

func TestLogDetailFallbackSanitizesUnsafeOneBotText(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)

	application.SetLogRepository(&stubMissingLogRepository{})
	application.Logs().Append(logging.Summary{
		LogID:     "log_live_only_unsafe_0001",
		Timestamp: "2026-04-09T20:51:46Z",
		Level:     "info",
		Source:    "bridge",
		Message:   "721011692: [860105388]群星怒\u2066~喵(3001): hello\u202eworld",
		RequestID: "dispatch_1775739204056693801",
		Details: map[string]any{
			"direction":       "inbound",
			"event_type":      "message.group",
			"self_id":         "721011692",
			"conversation_id": "860105388",
			"sender_nickname": "群星怒\u2066~喵",
			"plain_text":      "hello\u202eworld",
		},
	})

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/logs/log_live_only_unsafe_0001", nil)
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
	if body["message"] != "721011692: [860105388]群星怒~喵(3001): helloworld" {
		t.Fatalf("unexpected sanitized fallback message: %#v", body["message"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected fallback details map, got %#v", body["details"])
	}
	if details["plain_text"] != "helloworld" {
		t.Fatalf("unexpected sanitized fallback details: %#v", details)
	}
	if details["self_id"] != "721011692" {
		t.Fatalf("unexpected sanitized self_id detail: %#v", details["self_id"])
	}
	sender, ok := details["sender"].(map[string]any)
	if !ok {
		t.Fatalf("expected compacted sender map, got %#v", details["sender"])
	}
	if sender["nickname"] != "群星怒~喵" {
		t.Fatalf("unexpected sanitized fallback sender: %#v", sender)
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

func doLogsListRequest(t *testing.T, baseURL, token, requestPath string) map[string]any {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, baseURL+requestPath, nil)
	if err != nil {
		t.Fatalf("create logs list request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("perform logs list request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected logs list status: got %d want 200", response.StatusCode)
	}

	return decodeBody(t, readAll(t, response))
}

func putWhitelistState(t *testing.T, baseURL, token string, enabled bool) {
	t.Helper()

	body := `{"enabled":false}`
	if enabled {
		body = `{"enabled":true}`
	}

	request, err := http.NewRequest(http.MethodPut, baseURL+"/api/governance/whitelist/state", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create whitelist state request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("perform whitelist state request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected whitelist state status: got %d want 200", response.StatusCode)
	}
}

func commandRejectionEvent() adapter.NormalizedEvent {
	now := time.Now()
	return adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-command-rejected-help",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        now.Unix(),
		ConversationType: "private",
		ConversationID:   "20001",
		SenderID:         "30001",
		MessageID:        "90001",
		PlainText:        "/help",
		Segments: []adapter.MessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "/help"},
		}},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message",
				"message_type":   "private",
				"user_id":        "30001",
				"time":           now.Unix(),
				"message_id":     "90001",
				"raw_message":    "/help",
				"message_format": "array",
				"sender": map[string]any{
					"nickname": "Alice",
				},
			},
		},
	}
}

type stubMissingLogRepository struct{}

func (*stubMissingLogRepository) SaveSummary(context.Context, logging.Summary) error {
	return nil
}

func (*stubMissingLogRepository) ListSummaries(context.Context, logging.Query) ([]logging.Summary, error) {
	return nil, nil
}

func (*stubMissingLogRepository) ListPage(context.Context, logging.PageQuery) (logging.PageResult, error) {
	return logging.PageResult{}, nil
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

func TestLogsListCurrentSessionDoesNotCrossRestartBoundary(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	appA := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC) }, "logs-current-a")
	_ = issueLoginToken(t, appA)
	appA.Logger().Error(
		"old boot log should stay out of current session",
		"component", "runtime",
		"request_id", "req_current_old",
	)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 5, 0, 0, time.UTC) }, "logs-current-b")
	defer closePersistentTestApp(t, appB)
	tokenB := issueExistingBootstrapLoginToken(t, appB)
	appB.Logger().Error(
		"current boot log is visible",
		"component", "runtime",
		"request_id", "req_current_new",
	)

	serverB := httptest.NewServer(appB.Handler())
	defer serverB.Close()

	body := doLogsListRequest(t, serverB.URL, tokenB, "/api/logs?scope=current_session&limit=20")
	items := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected current session logs, got none")
	}

	foundCurrent := false
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["request_id"] == "req_current_old" {
			t.Fatalf("old boot log leaked into current session: %#v", item)
		}
		if item["request_id"] == "req_current_new" {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatalf("expected current boot log in current session response, got %#v", items)
	}
}

func TestLogsListReadsPersistedBridgeMessageAcrossRestart(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	appA := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 4, 15, 3, 0, 0, 0, time.UTC) }, "bridge-a")
	appA.SetBridge(newPersistentEventsBridge(appA))
	_ = issueLoginToken(t, appA)

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessageText,
		EventID:          "onebot11-message-899582563",
		BotID:            "1145141919",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Date(2026, 4, 14, 23, 59, 34, 0, time.FixedZone("CST", 8*3600)).Unix(),
		ConversationType: "group",
		ConversationID:   "553855023",
		SenderID:         "1358252269",
		PlainText:        "标题: 终末地困困小猫的一天 作者: 半截扣子w#32270458",
		MessageID:        "899582563",
		TargetName:       "终末地摸鱼群",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message",
				"message_type":   "group",
				"group_id":       "553855023",
				"user_id":        "1358252269",
				"time":           float64(1776009574),
				"message_id":     "899582563",
				"real_id":        "899582563",
				"message_seq":    "1306315",
				"raw_message":    "标题: 终末地困困小猫的一天 作者: 半截扣子w#32270458",
				"message_format": "array",
				"font":           float64(14),
				"sender": map[string]any{
					"nickname": "。",
					"card":     "群星怒，大明云玩家",
					"role":     "member",
					"title":    "管理员",
				},
			},
		},
	}

	appA.Bridge().HandleAdapterEvent(context.Background(), event)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 4, 15, 3, 5, 0, 0, time.UTC) }, "bridge-b")
	defer closePersistentTestApp(t, appB)

	tokenB := issueExistingBootstrapLoginToken(t, appB)
	serverB := httptest.NewServer(appB.Handler())
	defer serverB.Close()

	bridgeBody := doLogsListRequest(t, serverB.URL, tokenB, "/api/logs?source=bridge&limit=20")
	bridgeItems := bridgeBody["items"].([]any)
	if len(bridgeItems) == 0 {
		t.Fatalf("expected persisted bridge logs after restart, got none")
	}

	var bridgeItem map[string]any
	for _, raw := range bridgeItems {
		item := raw.(map[string]any)
		if item["source"] == "bridge" {
			bridgeItem = item
			break
		}
	}
	if bridgeItem == nil {
		t.Fatalf("expected a persisted bridge log after restart, got %#v", bridgeItems)
	}
	if !strings.Contains(bridgeItem["message"].(string), "1145141919: [终末地摸鱼群(553855023)][管理员]") {
		t.Fatalf("unexpected persisted bridge message: %#v", bridgeItem["message"])
	}

	protocolBody := doLogsListRequest(t, serverB.URL, tokenB, "/api/logs?protocol=onebot11&limit=20")
	protocolItems := protocolBody["items"].([]any)
	foundBridge := false
	for _, raw := range protocolItems {
		item := raw.(map[string]any)
		if item["source"] == "bridge" && strings.Contains(item["message"].(string), "1145141919: [终末地摸鱼群(553855023)][管理员]") {
			foundBridge = true
			break
		}
	}
	if !foundBridge {
		t.Fatalf("expected bridge message log in protocol history after restart, got %#v", protocolItems)
	}
}
