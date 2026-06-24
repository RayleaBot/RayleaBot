package integration

import (
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
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
			Message:   "echo plugin warning",
			PluginID:  "raylea.echo",
			RequestID: "req_echo_0001",
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
			Message:   "10001: [测试群(2001)]管理员/测试用户A(3001): hello bridge",
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
			Message:   "echo/echo -> 测试用户A(3001) 发送失败：hello world",
			PluginID:  "raylea.echo",
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
			Message:   "10001: [测试群(2001)]管理员/测试用户A(3001): hello bridge",
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
	if items[0].(map[string]any)["message"] != "10001: [测试群(2001)]管理员/测试用户A(3001): hello bridge" {
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
