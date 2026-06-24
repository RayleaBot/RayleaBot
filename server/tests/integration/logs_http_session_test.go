package integration

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessageText,
		EventID:          "onebot11-message-40002",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Date(2026, 4, 14, 23, 59, 34, 0, time.FixedZone("CST", 8*3600)).Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PlainText:        "标题: 测试动态标题 作者: 测试作者#40003",
		MessageID:        "40002",
		TargetName:       "测试群组",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message",
				"message_type":   "group",
				"group_id":       "20001",
				"user_id":        "30001",
				"time":           float64(1710000900),
				"message_id":     "40002",
				"real_id":        "40002",
				"message_seq":    "1306315",
				"raw_message":    "标题: 测试动态标题 作者: 测试作者#40003",
				"message_format": "array",
				"font":           float64(14),
				"sender": map[string]any{
					"nickname": "。",
					"card":     "测试群名片，测试用户昵称",
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
	if !strings.Contains(bridgeItem["message"].(string), "10001: [测试群组(20001)][管理员]") {
		t.Fatalf("unexpected persisted bridge message: %#v", bridgeItem["message"])
	}

	protocolBody := doLogsListRequest(t, serverB.URL, tokenB, "/api/logs?protocol=onebot11&limit=20")
	protocolItems := protocolBody["items"].([]any)
	foundBridge := false
	for _, raw := range protocolItems {
		item := raw.(map[string]any)
		if item["source"] == "bridge" && strings.Contains(item["message"].(string), "10001: [测试群组(20001)][管理员]") {
			foundBridge = true
			break
		}
	}
	if !foundBridge {
		t.Fatalf("expected bridge message log in protocol history after restart, got %#v", protocolItems)
	}
}
