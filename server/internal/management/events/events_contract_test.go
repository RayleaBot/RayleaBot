package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
)

func TestNewEventsReceivedFrameUsesFrozenEnvelope(t *testing.T) {
	t.Parallel()

	frame := NewReceivedFrame(GenericPayload{
		EventType: "governance.changed",
		Summary:   "治理设置已更新",
	})
	if frame.Channel != "events" {
		t.Fatalf("unexpected channel: got %q want %q", frame.Channel, "events")
	}
	if frame.Type != "events.received" {
		t.Fatalf("unexpected type: got %q want %q", frame.Type, "events.received")
	}
	if frame.Timestamp == "" {
		t.Fatal("timestamp should be populated")
	}

	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if payload["channel"] != "events" || payload["type"] != "events.received" {
		t.Fatalf("unexpected encoded frame: %s", encoded)
	}
}

func TestMessageStatsChangedFrameMatchesContractFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "fixtures", "websocket", "ok.events-received-message-stats-changed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Frame struct {
			Channel string         `json:"channel"`
			Type    string         `json:"type"`
			Data    map[string]any `json:"data"`
		} `json:"frame"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	service := NewMessageStatsService()
	notices, unsubscribe := service.Subscribe(1)
	defer unsubscribe()
	service.PublishChanged(messagestats.Change{
		ChangedAt:  time.Date(2026, 10, 1, 14, 30, 2, 0, time.FixedZone("fixture", 8*60*60)),
		AdapterIDs: []string{"onebot11", "qq-official"},
	})
	frame := <-notices
	if frame.Channel != fixture.Frame.Channel || frame.Type != fixture.Frame.Type {
		t.Fatalf("wrong envelope: %+v", frame)
	}
	if at, err := time.Parse(time.RFC3339Nano, frame.Timestamp); err != nil || at.Location() != time.UTC {
		t.Fatalf("invalid UTC timestamp: %q, %v", frame.Timestamp, err)
	}
	encoded, err := json.Marshal(frame.Data)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(payload, fixture.Frame.Data) {
		t.Fatalf("notice differs from contract fixture: %s", encoded)
	}
}

func TestPluginStateEventFrameKeepsContractFieldNames(t *testing.T) {
	t.Parallel()

	frame := NewReceivedFrame(PluginStatePayload{
		PluginID: "weather",
		State:    "running",
		Commands: []PluginCommandItem{
			{
				ID: "weather", Name: "weather", Description: "weather", Usage: "/weather",
				Permission: "everyone", Trigger: PluginCommandTrigger{Type: "exact", Names: []string{"weather"}},
			},
		},
		CommandConflicts: []string{},
	})

	encoded, err := json.Marshal(frame.Data)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	for _, key := range []string{"plugin_id", "state", "commands", "command_conflicts"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing field %q in %s", key, encoded)
		}
	}
}
