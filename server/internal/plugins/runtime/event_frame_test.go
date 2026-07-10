package runtime

import (
	"encoding/json"
	"testing"
)

func TestBuildEventFrameProjectsOneBotPayload(t *testing.T) {
	t.Parallel()

	frame := BuildEventFrame(Event{
		EventID:        "evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "onebot",
		EventType:      "message",
		Timestamp:      1700000000,
		MessageID:      "msg-1",
		Actor:          &EventActor{ID: "10001", Nickname: "Alice"},
		Target:         &EventTarget{Type: "group", ID: "20001"},
		Message:        &EventMessage{PlainText: "hello"},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":    "message",
				"message_type": "group",
				"group_id":     "20001",
				"user_id":      "10001",
			},
		},
	}, "weather", "req-1", 1700000001)

	if frame.ProtocolVersion != "1" || frame.PluginID != "weather" || frame.RequestID != "req-1" {
		t.Fatalf("unexpected frame identity: %#v", frame)
	}
	if frame.Event.Payload == nil || frame.Event.Payload.OneBot == nil {
		t.Fatalf("missing onebot payload: %#v", frame.Event.Payload)
	}
	if frame.Event.Payload.MessageID != "msg-1" || frame.Event.Payload.OneBot.GroupID != "20001" {
		t.Fatalf("unexpected onebot payload: %#v", frame.Event.Payload.OneBot)
	}
}

func TestBuildEventFrameProjectsWebhookMetadataAtEventRoot(t *testing.T) {
	t.Parallel()

	clientTimestamp := int64(1700000000)
	frame := BuildEventFrame(Event{
		EventID:        "webhook-event-1",
		SourceProtocol: "webhook",
		SourceAdapter:  "webhook.gateway",
		EventType:      "webhook.received",
		Timestamp:      1700000001,
		Webhook: &EventWebhook{
			Route:           "github",
			ReceivedAt:      1700000001,
			ClientTimestamp: &clientTimestamp,
			ClientEventID:   "github-delivery-1",
		},
	}, "repo-watcher", "request-1", 1700000002)

	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal event frame: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal event frame: %v", err)
	}
	eventDocument, ok := document["event"].(map[string]any)
	if !ok {
		t.Fatalf("event document = %#v", document["event"])
	}
	webhookDocument, ok := eventDocument["webhook"].(map[string]any)
	if !ok || webhookDocument["route"] != "github" {
		t.Fatalf("webhook document = %#v", eventDocument["webhook"])
	}
	if _, exists := eventDocument["payload"]; exists {
		t.Fatalf("webhook event unexpectedly contains payload: %#v", eventDocument["payload"])
	}
}
