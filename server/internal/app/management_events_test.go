package app

import (
	"encoding/json"
	"testing"
	"time"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
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

func TestBilibiliSourceStatusResponseIncludesDiagnosis(t *testing.T) {
	t.Parallel()

	retryAt := time.Date(2026, 6, 8, 8, 35, 0, 0, time.UTC)
	status := source.Status{
		Status:  source.StateDegraded,
		Summary: "Bilibili 事件源运行受限",
		Live: source.LiveStatus{
			WatchedRooms:    1,
			FallbackPolling: true,
			LastError:       "code -352",
		},
		Dynamic: source.DynamicStatus{
			Enabled:         true,
			IntervalSeconds: 10,
			WatchedUIDs:     1,
		},
		Diagnosis: source.Diagnosis{
			Level:       "attention",
			Headline:    "平台风控等待中",
			Description: "Bilibili 暂时限制部分请求，系统会在等待结束后自动恢复检查。",
			Causes: []source.DiagnosisCause{
				{
					Scope:     "live",
					Code:      "platform_risk_control",
					Title:     "直播请求被平台限制",
					Detail:    "直播状态检查暂时等待平台恢复。",
					LastError: "code -352",
					RetryAt:   &retryAt,
				},
			},
			Impacts:   []string{"动态接收不受影响。", "CK 有效，无需重新登录。"},
			Actions:   []source.DiagnosisAction{{Kind: "wait", Label: "等待平台恢复", Primary: true}},
			UpdatedAt: time.Date(2026, 6, 8, 8, 30, 0, 0, time.UTC),
		},
	}

	response := bilibiliSourceStatusResponseFrom(status)
	if response.Diagnosis.Level != "attention" || response.Diagnosis.Headline != "平台风控等待中" {
		t.Fatalf("unexpected diagnosis response: %#v", response.Diagnosis)
	}
	if len(response.Diagnosis.Causes) != 1 || response.Diagnosis.Causes[0].RetryAt == nil || *response.Diagnosis.Causes[0].RetryAt != "2026-06-08T08:35:00Z" {
		t.Fatalf("unexpected diagnosis causes: %#v", response.Diagnosis.Causes)
	}
	if len(response.Diagnosis.Actions) != 1 || response.Diagnosis.Actions[0].Kind != "wait" {
		t.Fatalf("unexpected diagnosis actions: %#v", response.Diagnosis.Actions)
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

	frame := bilibiliSourceStatusEventFrame(status)
	payload, ok := frame.Data.(bilibiliSourceEventPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", frame.Data)
	}
	if payload.Diagnosis.Headline != "直播备用检查中" || len(payload.Diagnosis.Causes) != 1 || payload.Diagnosis.Causes[0].Code != "live_fallback" {
		t.Fatalf("unexpected event diagnosis: %#v", payload.Diagnosis)
	}
}
