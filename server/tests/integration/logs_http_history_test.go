package integration

import (
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
			Message:   "历史范围外的较早日志样例",
		},
		{
			LogID:     "log_history_range_0001",
			Timestamp: "2026-03-20T00:05:00Z",
			Level:     "warn",
			Source:    "adapter.onebot11",
			Message:   "OneBot 主动 WebSocket 鉴权失败：ws://127.0.0.1:6700",
			RequestID: "req_adapter_0002",
		},
		{
			LogID:     "log_history_range_0002",
			Timestamp: "2026-03-20T10:00:01Z",
			Level:     "error",
			Source:    "runtime",
			Message:   "插件weather运行时 stderr 输出超过速率限制，已截断",
			PluginID:  "weather",
			RequestID: "req_plugin_0001",
		},
		{
			LogID:     "log_history_range_0003",
			Timestamp: "2026-03-20T20:12:00Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "插件 weather 的恢复状态摘要已刷新",
		},
		{
			LogID:     "log_history_range_ignored_0002",
			Timestamp: "2026-03-21T00:00:00Z",
			Level:     "info",
			Source:    "runtime",
			Message:   "历史范围外的较晚日志样例",
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
			Message:   "带时区偏移的日志仍在历史范围内",
		},
		{
			LogID:     "log_history_offset_0002",
			Timestamp: "2026-04-17T02:05:01+08:00",
			Level:     "info",
			Source:    "runtime",
			Message:   "带时区偏移的日志在历史范围外",
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
	if items[0].(map[string]any)["message"] != "带时区偏移的日志仍在历史范围内" {
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
		{LogID: "log_multi_cursor_0002", Timestamp: "2026-04-10T09:00:01Z", Level: "warn", Source: "runtime", Message: "按级别过滤掉的日志样例：raylea.echo", PluginID: "raylea.echo"},
		{LogID: "log_multi_cursor_0003", Timestamp: "2026-04-10T09:00:02Z", Level: "error", Source: "runtime", Message: "2", PluginID: "raylea.echo"},
		{LogID: "log_multi_cursor_0004", Timestamp: "2026-04-10T09:00:03Z", Level: "info", Source: "runtime", Message: "按插件过滤掉的日志样例：ops", PluginID: "ops"},
		{LogID: "log_multi_cursor_0005", Timestamp: "2026-04-10T09:00:04Z", Level: "error", Source: "runtime", Message: "3", PluginID: "weather"},
		{LogID: "log_multi_cursor_0006", Timestamp: "2026-04-10T09:00:05Z", Level: "info", Source: "runtime", Message: "4", PluginID: "raylea.echo"},
	} {
		application.Logs().Append(summary)
	}

	filterPath := "/api/logs?source=runtime&level=info&level=error&plugin_id=weather&plugin_id=raylea.echo&limit=2"
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
		"下游适配器握手拒绝 fixture-only-secret",
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
		Message:   "10001: [测试群(2001)]管理员/测试用户A(3001): hello bridge",
		RequestID: "req_bridge_0001",
		Details: map[string]any{
			"direction":         "inbound",
			"event_kind":        "onebot11.message",
			"event_type":        "message.group",
			"post_type":         "message",
			"message_type":      "group",
			"event_timestamp":   float64(1711015202),
			"self_id":           "10001",
			"time":              float64(1711015202),
			"conversation_type": "group",
			"conversation_id":   "2001",
			"group_name":        "测试群",
			"group_id":          "2001",
			"sender_id":         "3001",
			"user_id":           "3001",
			"sender_nickname":   "测试用户A",
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
				"nickname": "测试用户A",
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
