package integration

import (
	"context"
	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/segments"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

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
			"message_id":    "40001",
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

	configMutation := func(input map[string]any) {
		input["log"].(map[string]any)["retention_days"] = 365
	}
	application, _, _ := newTestAppWithOptions(t, configMutation, func(options *internalapp.Options, configPath string) {
		repoRoot := repoRootPath(t)
		options.PluginRepoRoot = repoRoot
		options.PluginSchemaPath = filepath.Join("..", "contracts", "plugin-info.schema.json")
		options.PluginRoots = []plugindiscovery.ScanRoot{
			{Label: "plugins/builtin", Path: filepath.Join(repoRoot, "plugins", "builtin")},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		}
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
		if item["message"] == "plugin raylea.echo command echo rejected by command policy: sender is not whitelisted" {
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
	if rejectionSummary["plugin_id"] != "raylea.echo" {
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
	if body["plugin_id"] != "raylea.echo" {
		t.Fatalf("unexpected command rejection detail plugin_id: %#v", body["plugin_id"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected command rejection details payload: %#v", body["details"])
	}
	if details["command_name"] != "echo" || details["error_code"] != "permission.not_whitelisted" {
		t.Fatalf("unexpected command rejection details: %#v", details)
	}
	if details["reason"] != "actor is not whitelisted" || details["policy_stage"] != "whitelist" {
		t.Fatalf("unexpected command rejection details: %#v", details)
	}
	if !reflect.DeepEqual(details["matched_plugin_ids"], []any{"raylea.echo"}) {
		t.Fatalf("unexpected matched_plugin_ids detail: %#v", details["matched_plugin_ids"])
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
		Message:   "10001: [测试群(20001)]测试用户A(3001): 测试消息内容",
		RequestID: "dispatch_1775739204056693800",
		Details: map[string]any{
			"direction":       "inbound",
			"event_type":      "message.group",
			"self_id":         "10001",
			"conversation_id": "20001",
			"group_name":      "测试群",
			"group_id":        "20001",
			"sender_id":       "3001",
			"sender_nickname": "测试用户A",
			"plain_text":      "测试消息内容",
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
	if details["plain_text"] != "测试消息内容" {
		t.Fatalf("unexpected fallback details: %#v", details)
	}
	if details["self_id"] != "10001" {
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
	if sender["user_id"] != "3001" || sender["nickname"] != "测试用户A" {
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
		Message:   "10001: [20001]测试群名片\u2066~喵(3001): hello\u202eworld",
		RequestID: "dispatch_1775739204056693801",
		Details: map[string]any{
			"direction":       "inbound",
			"event_type":      "message.group",
			"self_id":         "10001",
			"conversation_id": "20001",
			"sender_nickname": "测试群名片\u2066~喵",
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
	if body["message"] != "10001: [20001]测试群名片~喵(3001): helloworld" {
		t.Fatalf("unexpected sanitized fallback message: %#v", body["message"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected fallback details map, got %#v", body["details"])
	}
	if details["plain_text"] != "helloworld" {
		t.Fatalf("unexpected sanitized fallback details: %#v", details)
	}
	if details["self_id"] != "10001" {
		t.Fatalf("unexpected sanitized self_id detail: %#v", details["self_id"])
	}
	sender, ok := details["sender"].(map[string]any)
	if !ok {
		t.Fatalf("expected compacted sender map, got %#v", details["sender"])
	}
	if sender["nickname"] != "测试群名片~喵" {
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

func commandRejectionEvent() adapterintake.NormalizedEvent {
	now := time.Now()
	return adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-command-rejected-echo",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        now.Unix(),
		ConversationType: "private",
		ConversationID:   "20001",
		SenderID:         "30001",
		MessageID:        "90001",
		PlainText:        "/echo",
		Segments: []adaptersegments.MessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "/echo"},
		}},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message",
				"message_type":   "private",
				"user_id":        "30001",
				"time":           now.Unix(),
				"message_id":     "90001",
				"raw_message":    "/echo",
				"message_format": "array",
				"sender": map[string]any{
					"nickname": "测试用户A",
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
