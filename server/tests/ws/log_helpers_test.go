package ws

import (
	"net/http"
	"strings"
	"testing"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/segments"
)

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
