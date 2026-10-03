package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func TestBuildEventFramePreservesEmptyIdentitySnapshot(t *testing.T) {
	t.Parallel()
	frame := BuildEventFrame(chatevent.Event{
		EventID: "identities-empty", SourceProtocol: "platform", SourceAdapter: "adapters.internal",
		EventType: "bot.identities.changed", Timestamp: 1700000000,
		PayloadFields: map[string]any{"bots": []chatevent.BotIdentity{}},
	}, "req-identities", time.Now())
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

func TestBuildEventFrameProjectsSchedulerPayload(t *testing.T) {
	t.Parallel()

	deadline := time.UnixMilli(1_700_000_060_000)
	frame := BuildEventFrame(chatevent.Event{
		EventID:        "scheduler-subscription-hub-check-1",
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      1700000000,
		PayloadFields: map[string]any{
			"task_id": "subscription-hub-check",
			"action":  "check_subscriptions",
			"payload": map[string]any{
				"action": "check_subscriptions",
			},
		},
	}, "req-scheduler-1", deadline)

	if frame.Event.Payload == nil {
		t.Fatal("scheduler event payload is missing")
	}
	if frame.DeadlineAtMs != deadline.UnixMilli() || frame.Event.Payload.TaskID != "subscription-hub-check" {
		t.Fatalf("scheduler frame deadline=%d task_id=%q", frame.DeadlineAtMs, frame.Event.Payload.TaskID)
	}
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginwire.Validate(encoded, 0); err != nil {
		t.Fatalf("scheduler frame violates the protocol: %v", err)
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
	}, "req-config-empty-1", time.Now())

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

	frame := BuildEventFrame(chatevent.Event{
		EventID:        "webhook-event-1",
		SourceProtocol: "webhook",
		SourceAdapter:  "webhook.gateway",
		EventType:      "webhook.received",
		Timestamp:      1700000001,
		Webhook: &chatevent.Webhook{
			Route:      "github",
			ReceivedAt: 1700000001,
		},
	}, "request-1", time.Now())

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
