package runtime

import (
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

func TestBuildEventFramePreservesEmptyIdentitySnapshot(t *testing.T) {
	t.Parallel()
	frame := BuildEventFrame(chatevent.Event{
		EventID: "identities-empty", SourceProtocol: "platform", SourceAdapter: "adapters.internal",
		EventType: "bot.identities.changed", Timestamp: 1700000000,
		PayloadFields: map[string]any{"bots": []chatevent.BotIdentity{}},
	}, "req-identities")
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var received struct {
		Event struct {
			Payload struct {
				Bots *[]chatevent.BotIdentity `json:"bots"`
			} `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal(encoded, &received); err != nil {
		t.Fatal(err)
	}
	if received.Event.Payload.Bots == nil || len(*received.Event.Payload.Bots) != 0 {
		t.Fatalf("empty identity snapshot did not survive serialization: %s", encoded)
	}
}

func TestBuildEventFrameProjectsOneBotPayload(t *testing.T) {
	t.Parallel()

	frame := BuildEventFrame(chatevent.Event{
		EventID:        "evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "onebot",
		EventType:      "message",
		Timestamp:      1700000000,
		MessageID:      "msg-1",
		Actor:          &chatevent.Actor{ID: "10001", Nickname: "Alice"},
		Target:         &chatevent.Target{Type: "group", ID: "20001"},
		Message:        &chatevent.Message{PlainText: "hello"},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":    "message",
				"message_type": "group",
				"group_id":     "20001",
				"user_id":      "10001",
			},
		},
	}, "req-1")

	if frame.Type != "event" || frame.RequestID != "req-1" {
		t.Fatalf("unexpected frame identity: %#v", frame)
	}
	if frame.Event.Payload == nil || frame.Event.Payload.OneBot == nil {
		t.Fatalf("missing onebot payload: %#v", frame.Event.Payload)
	}
	if frame.Event.Payload.MessageID != "msg-1" || frame.Event.Payload.OneBot.GroupID != "20001" {
		t.Fatalf("unexpected onebot payload: %#v", frame.Event.Payload.OneBot)
	}
}

func TestBuildEventFrameProjectsSchedulerPayload(t *testing.T) {
	t.Parallel()

	frame := BuildEventFrame(chatevent.Event{
		EventID:        "scheduler-subscription-hub-check-1",
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      1700000000,
		PayloadFields: map[string]any{
			"action": "check_subscriptions",
			"payload": map[string]any{
				"action": "check_subscriptions",
			},
		},
	}, "req-scheduler-1")

	if frame.Event.Payload == nil {
		t.Fatal("scheduler event payload is missing")
	}
	if frame.Event.Payload.Action != "check_subscriptions" {
		t.Fatalf("scheduler action = %q, want check_subscriptions", frame.Event.Payload.Action)
	}
	if got := frame.Event.Payload.Payload["action"]; got != "check_subscriptions" {
		t.Fatalf("scheduler nested payload action = %#v, want check_subscriptions", got)
	}
}

func TestBuildEventFramePreservesEmptyConfigSnapshot(t *testing.T) {
	t.Parallel()

	frame := BuildEventFrame(chatevent.Event{
		EventID:        "config-empty-1",
		SourceProtocol: "system",
		SourceAdapter:  "config",
		EventType:      "config.changed",
		Timestamp:      1700000000,
		PayloadFields: map[string]any{
			"config":       map[string]any{},
			"changed_keys": []string{"removed_key"},
		},
	}, "req-config-empty-1")

	if frame.Event.Payload == nil || frame.Event.Payload.Config == nil || len(*frame.Event.Payload.Config) != 0 {
		t.Fatalf("empty config snapshot was not preserved: %#v", frame.Event.Payload)
	}
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal event frame: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal event frame: %v", err)
	}
	eventDocument := document["event"].(map[string]any)
	payloadDocument := eventDocument["payload"].(map[string]any)
	configDocument, ok := payloadDocument["config"].(map[string]any)
	if !ok || len(configDocument) != 0 {
		t.Fatalf("serialized config snapshot = %#v", payloadDocument["config"])
	}
}

func TestBuildEventFrameProjectsWebhookMetadataAtEventRoot(t *testing.T) {
	t.Parallel()

	clientTimestamp := int64(1700000000)
	frame := BuildEventFrame(chatevent.Event{
		EventID:        "webhook-event-1",
		SourceProtocol: "webhook",
		SourceAdapter:  "webhook.gateway",
		EventType:      "webhook.received",
		Timestamp:      1700000001,
		Webhook: &chatevent.Webhook{
			Route:           "github",
			ReceivedAt:      1700000001,
			ClientTimestamp: &clientTimestamp,
			ClientEventID:   "github-delivery-1",
		},
	}, "request-1")

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
