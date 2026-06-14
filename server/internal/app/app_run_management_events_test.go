package app

import (
	"encoding/json"
	"testing"
	"time"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
)

func TestNewEventsReceivedFrameUsesFrozenEnvelope(t *testing.T) {
	t.Parallel()

	frame := managementevents.NewReceivedFrame(managementevents.GenericPayload{
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

	frame := managementevents.NewReceivedFrame(managementevents.PluginStatePayload{
		PluginID:          "weather",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
		Commands: []managementevents.PluginCommandItem{
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

func TestBilibiliSourceStatusEventIncludesDiagnosis(t *testing.T) {
	t.Parallel()

	status := source.Status{
		Status:  source.StateDegraded,
		Summary: "Bilibili 事件源运行受限",
		Live: source.LiveStatus{
			WatchedRooms:    1,
			FallbackPolling: true,
			LastError:       "直播间连接失败",
		},
		Diagnosis: source.Diagnosis{
			Level:       "attention",
			Headline:    "直播备用检查中",
			Description: "部分直播长连接不可用，系统正在使用接口检查直播状态。",
			Causes: []source.DiagnosisCause{
				{Scope: "live", Code: "live_fallback", Title: "直播实时连接受限", Detail: "直播状态仍会检查。"},
			},
			Impacts:   []string{"直播状态仍会检查，但实时性可能降低。"},
			Actions:   []source.DiagnosisAction{{Kind: "restart_source", Label: "重启事件源", Primary: true}},
			UpdatedAt: time.Date(2026, 6, 8, 8, 30, 0, 0, time.UTC),
		},
	}

	frame := managementevents.BilibiliSourceStatusFrame(status)
	payload, ok := frame.Data.(managementevents.BilibiliSourcePayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", frame.Data)
	}
	if payload.Diagnosis.Headline != "直播备用检查中" || len(payload.Diagnosis.Causes) != 1 || payload.Diagnosis.Causes[0].Code != "live_fallback" {
		t.Fatalf("unexpected event diagnosis: %#v", payload.Diagnosis)
	}
}
