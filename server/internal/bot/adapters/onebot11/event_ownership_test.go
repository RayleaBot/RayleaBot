package onebot11

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
)

func TestInvalidFramePreviewOwnsItsInput(t *testing.T) {
	for _, raw := range []string{`{"broken":`, `{"echo":true}`} {
		payload := []byte(raw)
		frame := ClassifyFrame(websocket.MessageText, payload, time.Now())
		copy(payload, bytes.Repeat([]byte{'x'}, len(payload)))
		if string(frame.PayloadPreview) != raw {
			t.Fatal("invalid frame preview aliases caller bytes")
		}
	}
}

func TestEnrichedPayloadKeepsNestedInputIndependent(t *testing.T) {
	shell := New("fixture", config.OneBotConfig{}, config.AdapterConfig{}, nil)
	input := chatevent.NormalizedEvent{
		SourceProtocol: "onebot11", SourceAdapter: "fixture", EventType: "message.group", ConversationType: "group", ConversationID: "3", SenderID: "2",
		PayloadFields: map[string]any{
			"sender": map[string]any{"nickname": "fixture"},
			"onebot": map[string]any{"group_name": "group", "sender": map[string]any{"role": "member"}, "extension": []any{map[string]any{"value": "original"}}},
		},
	}
	result := shell.EnrichEventMetadata(context.Background(), input)
	result.PayloadFields["sender"].(map[string]any)["nickname"] = "changed"
	result.PayloadFields["onebot"].(map[string]any)["extension"].([]any)[0].(map[string]any)["value"] = "changed"
	if input.PayloadFields["sender"].(map[string]any)["nickname"] != "fixture" || input.PayloadFields["onebot"].(map[string]any)["extension"].([]any)[0].(map[string]any)["value"] != "original" {
		t.Fatal("metadata enrichment mutated the input")
	}
	if result.ActorRole != "member" || result.TargetName != "group" {
		t.Fatal("metadata merge changed")
	}
}
