package app

import (
	"encoding/json"
	"testing"
)

func TestNewEventsReceivedFrameUsesFrozenEnvelope(t *testing.T) {
	t.Parallel()

	frame := newEventsReceivedFrame(genericManagementEventPayload{
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

func TestPluginStateEventFrameKeepsContractFieldNames(t *testing.T) {
	t.Parallel()

	frame := newEventsReceivedFrame(pluginStateEventPayload{
		PluginID:          "weather",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
		Commands: []pluginCommandEventItem{
			{
				Name:          "weather",
				CommandSource: "manifest",
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
	for _, key := range []string{"plugin_id", "registration_state", "desired_state", "runtime_state", "display_state", "commands", "command_conflicts"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing field %q in %s", key, encoded)
		}
	}
}
