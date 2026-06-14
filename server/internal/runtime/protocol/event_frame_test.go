package runtimeprotocol

import "testing"

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
