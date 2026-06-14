package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/spec"
)

func TestManagerStartInitAckSuccess(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "success", recordPath)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.State != StateRunning {
		t.Fatalf("unexpected state: got %q want %q", snapshot.State, StateRunning)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) == 0 {
		t.Fatalf("expected recorded init frame")
	}
	if frames[0]["type"] != "init" {
		t.Fatalf("unexpected first frame type: %v", frames[0]["type"])
	}
	commandPrefixes, ok := frames[0]["command_prefixes"].([]any)
	if !ok || len(commandPrefixes) != 2 || commandPrefixes[0] != "!" || commandPrefixes[1] != "/" {
		t.Fatalf("unexpected init command_prefixes: %#v", frames[0]["command_prefixes"])
	}
	permissions, ok := frames[0]["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("missing init permissions: %#v", frames[0])
	}
	superAdmins, ok := permissions["super_admins"].([]any)
	if !ok || len(superAdmins) != 2 || superAdmins[0] != "9001" || superAdmins[1] != "9002" {
		t.Fatalf("unexpected init super_admins: %#v", permissions["super_admins"])
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartOmitsBotWhenUnavailable(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "success", recordPath)
	payload := testInitPayload()
	payload.Bot = runtimespec.BotInfo{}

	if err := manager.Start(context.Background(), spec, payload); err != nil {
		t.Fatalf("start runtime without bot identity: %v", err)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) == 0 {
		t.Fatalf("expected recorded init frame")
	}
	if _, ok := frames[0]["bot"]; ok {
		t.Fatalf("init frame should omit bot when identity is unavailable: %#v", frames[0])
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartAllowsInitProgressBeforeReady(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "progress-then-ready", "", 2*time.Second, 4*time.Second, 400*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with init_progress: %v", err)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartStoresInitAckSubscriptions(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "success", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Fatalf("stop runtime: %v", err)
		}
	}()

	snapshot := manager.Snapshot()
	if len(snapshot.Subscriptions) != 2 {
		t.Fatalf("unexpected subscriptions: %#v", snapshot.Subscriptions)
	}
	if snapshot.Subscriptions[0] != "message.group" || snapshot.Subscriptions[1] != "scheduler.trigger" {
		t.Fatalf("unexpected subscriptions: %#v", snapshot.Subscriptions)
	}
}

func TestManagerStartFailsOnInitAckTimeout(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "timeout", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInitTimeout)

	snapshot := manager.Snapshot()
	if snapshot.State != StateStopped {
		t.Fatalf("unexpected state after timeout: got %q want %q", snapshot.State, StateStopped)
	}
}

func TestManagerStartFailsWhenInitExceedsMaxTotal(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "progress-forever", "", 200*time.Millisecond, 350*time.Millisecond, 400*time.Millisecond)

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInitTimeout)
}

func TestManagerStartFailsOnProtocolViolation(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "wrong-type", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerStartFailsOnEarlyExit(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "early-exit", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestManagerStartSucceedsWithLargeStderrOutput(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "stderr-noise", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with stderr noise: %v", err)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerGracefulStop(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "success", recordPath)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.State != StateStopped {
		t.Fatalf("unexpected state after stop: got %q want %q", snapshot.State, StateStopped)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) < 2 {
		t.Fatalf("expected init and shutdown frames, got %d", len(frames))
	}
	if frames[1]["type"] != "shutdown" {
		t.Fatalf("unexpected second frame type: %v", frames[1]["type"])
	}
}

func TestManagerDeliverEventReturnsResult(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "event-result", recordPath)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if delivery.RequestID == "" {
		t.Fatal("expected request id on successful delivery")
	}
	if handled, _ := delivery.Result["handled"].(bool); !handled {
		t.Fatalf("unexpected delivery result: %#v", delivery.Result)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) < 2 {
		t.Fatalf("expected init and event frames, got %d", len(frames))
	}
	if frames[1]["type"] != "event" {
		t.Fatalf("unexpected event frame type: %#v", frames[1]["type"])
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestBuildEventFrameIncludesOneBotPayload(t *testing.T) {
	t.Parallel()

	frame := runtimeprotocol.BuildEventFrame(runtimeprotocol.Event{
		EventID:        "evt-onebot-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message_sent.group",
		Timestamp:      1_729_679_125,
		Actor: &runtimeprotocol.EventActor{
			ID:       "10001",
			Nickname: "--",
			Role:     "owner",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "group",
			ID:   "20001",
		},
		Message: &runtimeprotocol.EventMessage{
			PlainText: "您好",
			Segments: []runtimeprotocol.EventSegment{{
				Type: "text",
				Data: map[string]any{"text": "您好"},
			}},
		},
		MessageID: "40001",
		PayloadFields: map[string]any{
			"sub_type": "normal",
			"onebot": map[string]any{
				"post_type":      "message_sent",
				"message_type":   "group",
				"self_id":        "10001",
				"user_id":        "10001",
				"group_id":       "20001",
				"time":           int64(1_729_679_125),
				"message_id":     "40001",
				"real_id":        "40001",
				"message_seq":    "40001",
				"raw_message":    "您好",
				"font":           14,
				"message_format": "array",
				"sender": map[string]any{
					"user_id":  "10001",
					"nickname": "--",
					"role":     "owner",
				},
			},
		},
	}, "echo", "req_evt_onebot", 1_729_679_126)

	if frame.Event.Payload == nil || frame.Event.Payload.OneBot == nil {
		t.Fatalf("expected onebot payload, got %#v", frame.Event.Payload)
	}
	if frame.Event.Payload.MessageID != "40001" {
		t.Fatalf("unexpected payload message_id: %#v", frame.Event.Payload.MessageID)
	}
	if frame.Event.Payload.OneBot.PostType != "message_sent" {
		t.Fatalf("unexpected onebot post_type: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.GroupID != "20001" {
		t.Fatalf("unexpected onebot group_id: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.Time != 1_729_679_125 {
		t.Fatalf("unexpected onebot time: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.Sender == nil || frame.Event.Payload.OneBot.Sender.Nickname != "--" {
		t.Fatalf("unexpected onebot sender: %#v", frame.Event.Payload.OneBot.Sender)
	}
}

func TestBuildEventFrameIncludesBilibiliRichDynamicPayload(t *testing.T) {
	t.Parallel()

	frame := runtimeprotocol.BuildEventFrame(runtimeprotocol.Event{
		EventID:        "evt-bilibili-rich-1",
		SourceProtocol: "bilibili",
		SourceAdapter:  "bilibili.source",
		EventType:      "bilibili.dynamic.published",
		Timestamp:      1_729_679_140,
		PayloadFields: map[string]any{
			"bilibili": map[string]any{
				"kind":         "dynamic",
				"uid":          "123456",
				"id":           "90002",
				"service":      "repost",
				"title":        "转发动态",
				"summary":      "转发说明",
				"summary_html": `<span class="rich-text-topic">#活动#</span>`,
				"url":          "https://t.bilibili.com/90002/",
				"topic": map[string]any{
					"id":       int64(1156147),
					"name":     "测试活动 2026",
					"jump_url": "https://m.bilibili.com/topic-detail?topic_id=1156147",
				},
				"author": map[string]any{
					"uid":  "123456",
					"name": "测试 UP",
				},
				"original": map[string]any{
					"id":           "80001",
					"service":      "image_text",
					"title":        "图文动态更新",
					"summary":      "原动态正文",
					"summary_html": `<span class="rich-text-topic">#原动态#</span>`,
					"url":          "https://t.bilibili.com/80001/",
					"pub_ts":       int64(1_729_679_100),
					"created_at":   "2024-10-23 12:00",
					"dynamic_type": "DYNAMIC_TYPE_DRAW",
					"topic": map[string]any{
						"id":       int64(10001),
						"name":     "原动态话题",
						"jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
					},
					"author": map[string]any{
						"uid":  "654321",
						"name": "原作者",
					},
					"images": []any{
						map[string]any{
							"url":    "https://i0.hdslb.com/bfs/new_dyn/original.jpg",
							"width":  900,
							"height": 1600,
						},
					},
				},
			},
		},
	}, "raylea.subscription-hub", "evt_bilibili_dynamic_rich_001", 1_729_679_141)

	if frame.Event.Payload == nil || frame.Event.Payload.Bilibili == nil {
		t.Fatalf("expected bilibili payload, got %#v", frame.Event.Payload)
	}
	payload := frame.Event.Payload.Bilibili
	if payload.SummaryHTML == "" || payload.Original == nil {
		t.Fatalf("unexpected bilibili rich payload: %#v", payload)
	}
	if payload.Topic == nil || payload.Topic.Name != "测试活动 2026" || payload.Topic.ID != 1156147 {
		t.Fatalf("unexpected bilibili topic payload: %#v", payload.Topic)
	}
	if payload.Original.Author.UID != "654321" || payload.Original.SummaryHTML == "" {
		t.Fatalf("unexpected bilibili original payload: %#v", payload.Original)
	}
	if payload.Original.Topic == nil || payload.Original.Topic.Name != "原动态话题" || payload.Original.Topic.ID != 10001 {
		t.Fatalf("unexpected bilibili original topic payload: %#v", payload.Original.Topic)
	}
	if len(payload.Original.Images) != 1 || payload.Original.Images[0].Height != 1600 {
		t.Fatalf("unexpected bilibili original images: %#v", payload.Original.Images)
	}
}

func TestBuildEventFrameIncludesMetaOneBotPayload(t *testing.T) {
	t.Parallel()

	frame := runtimeprotocol.BuildEventFrame(runtimeprotocol.Event{
		EventID:        "evt-onebot-meta-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "meta.heartbeat",
		Timestamp:      1_729_679_130,
		Actor: &runtimeprotocol.EventActor{
			ID: "10001",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "bot",
			ID:   "10001",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":       "meta_event",
				"meta_event_type": "heartbeat",
				"self_id":         "10001",
				"time":            int64(1_729_679_130),
				"interval":        5000,
				"status": map[string]any{
					"online": true,
				},
			},
		},
	}, "echo", "req_evt_onebot_meta", 1_729_679_131)

	if frame.Event.Payload == nil || frame.Event.Payload.OneBot == nil {
		t.Fatalf("expected onebot payload, got %#v", frame.Event.Payload)
	}
	if frame.Event.Payload.OneBot.MetaEventType != "heartbeat" {
		t.Fatalf("unexpected meta_event_type: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.Interval != 5000 {
		t.Fatalf("unexpected interval: %#v", frame.Event.Payload.OneBot)
	}
	if got := frame.Event.Payload.OneBot.Status["online"]; got != true {
		t.Fatalf("unexpected status payload: %#v", frame.Event.Payload.OneBot.Status)
	}
}

func TestManagerDeliverEventReturnsPluginError(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-error", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginNotHandled)
	if delivery.ErrorCode != codePluginNotHandled {
		t.Fatalf("unexpected delivery error code: got %q want %q", delivery.ErrorCode, codePluginNotHandled)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventReturnsPluginErrorDetails(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-error-with-details", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginNotHandled)
	if got, ok := delivery.ErrorDetails["reason"].(string); !ok || got != "policy_skip" {
		t.Fatalf("unexpected delivery error details: %#v", delivery.ErrorDetails)
	}

	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %T", err)
	}
	if got, ok := runtimeErr.Details["reason"].(string); !ok || got != "policy_skip" {
		t.Fatalf("unexpected runtime error details: %#v", runtimeErr.Details)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventReturnsAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-action-message-send", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if delivery.Action == nil {
		t.Fatalf("expected outbound action delivery, got %#v", delivery)
	}
	if delivery.Action.Kind != "message.send" {
		t.Fatalf("unexpected action kind: got %q want %q", delivery.Action.Kind, "message.send")
	}
	if delivery.Action.TargetType != "group" || delivery.Action.TargetID != "2001" {
		t.Fatalf("unexpected action payload: %#v", delivery.Action)
	}
	if len(delivery.Action.MessageSegments) != 1 || delivery.Action.MessageSegments[0].Type != "text" || delivery.Action.MessageSegments[0].Data["text"] != "hello from plugin" {
		t.Fatalf("unexpected action segments: %#v", delivery.Action.MessageSegments)
	}
	if delivery.Result != nil {
		t.Fatalf("did not expect result payload alongside action: %#v", delivery.Result)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventReturnsRichMessageSendAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-action-message-send-segments", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if delivery.Action == nil {
		t.Fatalf("expected outbound action delivery, got %#v", delivery)
	}
	if len(delivery.Action.MessageSegments) != 3 {
		t.Fatalf("unexpected message segments: %#v", delivery.Action.MessageSegments)
	}
	if delivery.Action.MessageSegments[0].Type != "at" || delivery.Action.MessageSegments[1].Type != "text" || delivery.Action.MessageSegments[2].Type != "image" {
		t.Fatalf("unexpected rich action payload: %#v", delivery.Action)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventReturnsRichMessageReplyAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-action-message-reply-to-event", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if delivery.Action == nil {
		t.Fatalf("expected outbound action delivery, got %#v", delivery)
	}
	if delivery.Action.ReplyToEventID != "onebot11-message-12345" {
		t.Fatalf("unexpected rich reply action payload: %#v", delivery.Action)
	}
	if !delivery.Action.FallbackToSendIfMissing {
		t.Fatalf("expected fallback flag on rich reply: %#v", delivery.Action)
	}
	if len(delivery.Action.MessageSegments) != 1 || delivery.Action.MessageSegments[0].Type != "text" {
		t.Fatalf("unexpected rich reply segments: %#v", delivery.Action.MessageSegments)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestParseMessageSendActionRejectsRemovedTextPayload(t *testing.T) {
	t.Parallel()

	_, err := runtimeaction.ParseTerminalAction("message.send", json.RawMessage(`{
		"target_type": "group",
		"target_id": "2001",
		"text": "removed text payload"
	}`))
	assertActionErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerDeliverEventProcessesLocalActionsBeforeTerminalResult(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		actions []runtimeaction.Action
	)
	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
			mu.Lock()
			actions = append(actions, action)
			mu.Unlock()

			if pluginID != "helper-plugin" {
				t.Fatalf("pluginID = %q, want helper-plugin", pluginID)
			}
			if parentEvent.EventID != "evt-1" || parentEvent.Target == nil || parentEvent.Target.ID != "2001" {
				t.Fatalf("unexpected parent event: %#v", parentEvent)
			}
			switch requestID {
			case "local_logger_1":
				return map[string]any{}, nil
			case "local_storage_1":
				return map[string]any{
					"key":    action.StorageKey,
					"exists": true,
					"value": map[string]any{
						"count": 3,
					},
				}, nil
			default:
				t.Fatalf("unexpected local request_id: %q", requestID)
				return nil, nil
			}
		},
	})
	spec := helperSpec(t, "event-local-actions-then-result", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if handled, _ := delivery.Result["handled"].(bool); !handled {
		t.Fatalf("unexpected delivery result: %#v", delivery.Result)
	}
	if got, _ := delivery.Result["storage_exists"].(bool); !got {
		t.Fatalf("expected storage_exists=true, got %#v", delivery.Result)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 2 {
		t.Fatalf("len(actions) = %d, want 2", len(actions))
	}
	if actions[0].Kind != "logger.write" || actions[0].LogMessage != "notice.member_increase received" {
		t.Fatalf("unexpected first local action: %#v", actions[0])
	}
	if actions[1].Kind != "storage.kv" || actions[1].StorageOperation != "get" || actions[1].StorageKey != "notice:last_join" {
		t.Fatalf("unexpected second local action: %#v", actions[1])
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventWritesLocalActionErrorAndContinues(t *testing.T) {
	t.Parallel()

	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(_ context.Context, _ string, _ string, action runtimeaction.Action, _ runtimeprotocol.Event) (map[string]any, error) {
			if action.Kind != "logger.write" {
				t.Fatalf("unexpected local action: %#v", action)
			}
			return nil, errorf("permission.scope_violation", "capability not granted", nil)
		},
	})
	spec := helperSpec(t, "event-local-action-error-then-result", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	if got, _ := delivery.Result["local_error_code"].(string); got != "permission.scope_violation" {
		t.Fatalf("local_error_code = %q, want %q", got, "permission.scope_violation")
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventWritesLocalActionErrorDetailsAndContinues(t *testing.T) {
	t.Parallel()

	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(_ context.Context, _ string, _ string, action runtimeaction.Action, _ runtimeprotocol.Event) (map[string]any, error) {
			if action.Kind != "logger.write" {
				t.Fatalf("unexpected local action: %#v", action)
			}
			return nil, &Error{
				Code:    "permission.scope_violation",
				Message: "capability not granted",
				Details: map[string]any{
					"missing_capability": "logger.write",
					"scope":              "management.logs:write",
				},
			}
		},
	})
	spec := helperSpec(t, "event-local-action-error-then-result", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	if err != nil {
		t.Fatalf("deliver event: %v", err)
	}
	details, ok := delivery.Result["local_error_details"].(map[string]any)
	if !ok {
		t.Fatalf("expected local_error_details map, got %#v", delivery.Result["local_error_details"])
	}
	if details["missing_capability"] != "logger.write" {
		t.Fatalf("unexpected local error details: %#v", details)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventRejectsLocalActionWithoutParentRequestIDWhenConcurrent(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithConcurrency(t, "event-local-action-missing-parent-request-id", "", 2)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventProcessesConcurrentLocalActionsWithinOneSession(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	release := make(chan struct{})
	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action runtimeaction.Action, _ runtimeprotocol.Event) (map[string]any, error) {
			if pluginID != "helper-plugin" {
				t.Fatalf("pluginID = %q, want helper-plugin", pluginID)
			}
			started <- requestID
			<-release
			return map[string]any{"request_id": requestID}, nil
		},
	})
	spec := helperSpecWithConcurrency(t, "event-concurrent-local-actions-then-result", "", 2)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	type deliveryResult struct {
		delivery Delivery
		err      error
	}
	done := make(chan deliveryResult, 1)
	go func() {
		delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
		done <- deliveryResult{delivery: delivery, err: err}
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case requestID := <-started:
			seen[requestID] = true
		case <-time.After(runtimeTestDuration(500 * time.Millisecond)):
			t.Fatalf("expected two local actions to start concurrently, got %#v", seen)
		}
	}
	if !seen["local_logger_3"] || !seen["local_storage_3"] {
		t.Fatalf("unexpected local action request ids: %#v", seen)
	}

	close(release)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("deliver event: %v", result.err)
		}
		if got, _ := result.delivery.Result["logger_started"].(bool); !got {
			t.Fatalf("unexpected logger_started result: %#v", result.delivery.Result)
		}
		if got, _ := result.delivery.Result["storage_started"].(bool); !got {
			t.Fatalf("unexpected storage_started result: %#v", result.delivery.Result)
		}
	case <-time.After(runtimeTestDuration(time.Second)):
		t.Fatal("deliver event did not finish after local actions completed")
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventRejectsTerminalFrameBeforePendingLocalActionsComplete(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(context.Context, string, string, runtimeaction.Action, runtimeprotocol.Event) (map[string]any, error) {
			<-release
			return map[string]any{}, nil
		},
	})
	spec := helperSpecWithConcurrency(t, "event-local-action-early-terminal-result", "", 2)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
	close(release)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventRejectsLocalActionUsingEventRequestID(t *testing.T) {
	t.Parallel()

	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(context.Context, string, string, runtimeaction.Action, runtimeprotocol.Event) (map[string]any, error) {
			t.Fatal("ExecuteLocalAction should not be called when request_id reuses the event request_id")
			return nil, nil
		},
	})
	spec := helperSpec(t, "event-local-action-same-request-id", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventConcurrentSessionsDoNotBlockOnSlowLocalAction(t *testing.T) {
	t.Parallel()

	startedSlow := make(chan string, 1)
	releaseSlow := make(chan struct{})
	manager := testManagerWithRequestIDs(Options{
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action runtimeaction.Action, _ runtimeprotocol.Event) (map[string]any, error) {
			if pluginID != "helper-plugin" {
				t.Fatalf("pluginID = %q, want helper-plugin", pluginID)
			}
			if action.Kind != "http.request" {
				t.Fatalf("unexpected local action kind: %#v", action)
			}
			startedSlow <- requestID
			<-releaseSlow
			return map[string]any{"status_code": 200}, nil
		},
	})
	spec := helperSpecWithConcurrency(t, "event-concurrent-slow-local-action-does-not-block-other-session", "", 2)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	type deliveryResult struct {
		delivery Delivery
		err      error
	}

	firstDone := make(chan deliveryResult, 1)
	go func() {
		delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEventWithTarget("2001"))
		firstDone <- deliveryResult{delivery: delivery, err: err}
	}()

	select {
	case requestID := <-startedSlow:
		if requestID != "slow_http_1" {
			t.Fatalf("unexpected slow request_id: %q", requestID)
		}
	case <-time.After(runtimeTestDuration(500 * time.Millisecond)):
		t.Fatal("expected slow local action to start")
	}

	secondDone := make(chan deliveryResult, 1)
	go func() {
		delivery, err := manager.DeliverEvent(context.Background(), testRuntimeEventWithTarget("2002"))
		secondDone <- deliveryResult{delivery: delivery, err: err}
	}()

	select {
	case result := <-secondDone:
		if result.err != nil {
			t.Fatalf("second deliver event: %v", result.err)
		}
		if got, _ := result.delivery.Result["session"].(string); got != "fast" {
			t.Fatalf("unexpected fast session result: %#v", result.delivery.Result)
		}
	case <-time.After(runtimeTestDuration(500 * time.Millisecond)):
		t.Fatal("second session remained blocked behind the slow local action")
	}

	close(releaseSlow)

	select {
	case result := <-firstDone:
		if result.err != nil {
			t.Fatalf("first deliver event: %v", result.err)
		}
		if got, _ := result.delivery.Result["session"].(string); got != "slow" {
			t.Fatalf("unexpected slow session result: %#v", result.delivery.Result)
		}
	case <-time.After(runtimeTestDuration(time.Second)):
		t.Fatal("first session did not finish after slow local action completed")
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-unsupported-action", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestParseStorageFileActionWriteText(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("storage.file", json.RawMessage(`{
		"operation": "write",
		"root": "plugin_data",
		"path": "cache/example.txt",
		"content_text": "hello file"
	}`))
	if err != nil {
		t.Fatalf("parseStorageFileAction: %v", err)
	}
	if action.Kind != "storage.file" || action.StorageOperation != "write" || action.StoragePath != "cache/example.txt" {
		t.Fatalf("unexpected storage.file action: %#v", action)
	}
	if string(action.StorageContent) != "hello file" {
		t.Fatalf("unexpected storage content: %#v", action.StorageContent)
	}
}

func TestParseHTTPRequestActionRejectsGetWithBody(t *testing.T) {
	t.Parallel()

	_, err := runtimeaction.ParseLocalAction("http.request", json.RawMessage(`{
		"method": "GET",
		"url": "https://api.example.test/v1/data",
		"body_text": "denied"
	}`))
	assertActionErrorCode(t, err, codePluginProtocolViolation)
}

func TestParseConfigReadAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("config.read", json.RawMessage(`{
		"keys": ["default_city", "unit"]
	}`))
	if err != nil {
		t.Fatalf("parseConfigReadAction: %v", err)
	}
	if action.Kind != "config.read" || len(action.ConfigKeys) != 2 {
		t.Fatalf("unexpected config.read action: %#v", action)
	}
}

func TestParsePluginListActionVisibility(t *testing.T) {
	t.Parallel()

	catalog, err := runtimeaction.ParseLocalAction("plugin.list", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("parsePluginListAction catalog: %v", err)
	}
	if catalog.Kind != "plugin.list" || catalog.PluginListVisibility != "catalog" {
		t.Fatalf("unexpected plugin.list catalog action: %#v", catalog)
	}

	caller, err := runtimeaction.ParseLocalAction("plugin.list", json.RawMessage(`{"visibility":"caller"}`))
	if err != nil {
		t.Fatalf("parsePluginListAction caller: %v", err)
	}
	if caller.PluginListVisibility != "caller" {
		t.Fatalf("unexpected plugin.list caller action: %#v", caller)
	}

	_, err = runtimeaction.ParseLocalAction("plugin.list", json.RawMessage(`{"visibility":"invalid"}`))
	assertActionErrorCode(t, err, codePluginProtocolViolation)

	_, err = runtimeaction.ParseLocalAction("plugin.list", json.RawMessage(`{"visibility":"caller","extra":true}`))
	assertActionErrorCode(t, err, codePluginProtocolViolation)
}

func TestParseSecretReadAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("secret.read", json.RawMessage(`{
		"key": "bili_token_primary"
	}`))
	if err != nil {
		t.Fatalf("parseSecretReadAction: %v", err)
	}
	if action.Kind != "secret.read" || action.SecretKey != "bili_token_primary" {
		t.Fatalf("unexpected secret.read action: %#v", action)
	}
}

func TestParseConfigWriteAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("config.write", json.RawMessage(`{
		"values": {
			"default_city": "北京",
			"unit": "celsius"
		}
	}`))
	if err != nil {
		t.Fatalf("parseConfigWriteAction: %v", err)
	}
	if action.Kind != "config.write" || action.ConfigValues["default_city"] != "北京" {
		t.Fatalf("unexpected config.write action: %#v", action)
	}
}

func TestParseGovernanceBlacklistReadAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("governance.blacklist.read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("parseGovernanceBlacklistReadAction: %v", err)
	}
	if action.Kind != "governance.blacklist.read" {
		t.Fatalf("unexpected governance.blacklist.read action: %#v", action)
	}
}

func TestParseGovernanceBlacklistWriteAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("governance.blacklist.write", json.RawMessage(`{
		"operation": "upsert",
		"entry_type": "user",
		"target_id": "10001",
		"reason": "manual_review"
	}`))
	if err != nil {
		t.Fatalf("parseGovernanceBlacklistWriteAction: %v", err)
	}
	if action.Kind != "governance.blacklist.write" || action.GovernanceOperation != "upsert" || action.GovernanceTargetID != "10001" {
		t.Fatalf("unexpected governance.blacklist.write action: %#v", action)
	}
}

func TestParseGovernanceWhitelistWriteAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("governance.whitelist.write", json.RawMessage(`{
		"operation": "set_enabled",
		"enabled": true
	}`))
	if err != nil {
		t.Fatalf("parseGovernanceWhitelistWriteAction: %v", err)
	}
	if action.Kind != "governance.whitelist.write" || action.GovernanceOperation != "set_enabled" || action.GovernanceEnabled == nil || !*action.GovernanceEnabled {
		t.Fatalf("unexpected governance.whitelist.write action: %#v", action)
	}
}

func TestParseGovernanceCommandPolicyReadAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("governance.command_policy.read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("parseGovernanceCommandPolicyReadAction: %v", err)
	}
	if action.Kind != "governance.command_policy.read" {
		t.Fatalf("unexpected governance.command_policy.read action: %#v", action)
	}
}

func TestParseSchedulerCreateAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("scheduler.create", json.RawMessage(`{
		"task_id": "daily_report",
		"log_label": "每日早报",
		"cron": "0 8 * * *",
		"event_type": "scheduler.trigger",
		"payload": {
			"topic": "daily_report"
		}
	}`))
	if err != nil {
		t.Fatalf("parseSchedulerCreateAction: %v", err)
	}
	if action.Kind != "scheduler.create" || action.SchedulerTaskID != "daily_report" || action.SchedulerCron != "0 8 * * *" {
		t.Fatalf("unexpected scheduler.create action: %#v", action)
	}
	if action.SchedulerLogLabel != "每日早报" {
		t.Fatalf("SchedulerLogLabel = %q, want 每日早报", action.SchedulerLogLabel)
	}
}

func TestParseEventExposeWebhookAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("event.expose_webhook", json.RawMessage(`{
		"route": "github",
		"methods": ["POST"],
		"auth_strategy": "hmac_sha256",
		"header": "X-Hub-Signature-256",
		"signature_prefix": "sha256=",
		"secret_ref": "webhook.github.secret",
		"source_ips": ["192.0.2.0/24"],
		"replay_protection": {
			"timestamp_header": "X-Raylea-Timestamp",
			"event_id_header": "X-Raylea-runtimeprotocol.Event-Id",
			"tolerance_seconds": 300,
			"enforce": true
		}
	}`))
	if err != nil {
		t.Fatalf("parseEventExposeWebhookAction: %v", err)
	}
	if action.Kind != "event.expose_webhook" || action.WebhookRoute != "github" || action.WebhookAuthStrategy != "hmac_sha256" {
		t.Fatalf("unexpected event.expose_webhook action: %#v", action)
	}
	if len(action.WebhookMethods) != 1 || action.WebhookMethods[0] != "POST" {
		t.Fatalf("unexpected webhook methods: %#v", action.WebhookMethods)
	}
	if action.WebhookReplayProtection == nil {
		t.Fatalf("missing replay_protection on action")
	}
	if action.WebhookReplayProtection.TimestampHeader != "X-Raylea-Timestamp" ||
		action.WebhookReplayProtection.EventIDHeader != "X-Raylea-runtimeprotocol.Event-Id" ||
		action.WebhookReplayProtection.ToleranceSeconds != 300 ||
		!action.WebhookReplayProtection.Enforce {
		t.Fatalf("unexpected replay_protection: %+v", action.WebhookReplayProtection)
	}
}

func TestParseEventExposeWebhookActionRejectsMissingReplayProtection(t *testing.T) {
	t.Parallel()

	if _, err := runtimeaction.ParseLocalAction("event.expose_webhook", json.RawMessage(`{
		"route": "github",
		"methods": ["POST"],
		"auth_strategy": "hmac_sha256",
		"header": "X-Hub-Signature-256",
		"signature_prefix": "sha256=",
		"secret_ref": "webhook.github.secret"
	}`)); err == nil {
		t.Fatalf("expected error when replay_protection is missing")
	}
}

func TestParseRenderImageAction(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("render.image", json.RawMessage(`{
		"template": "help.menu",
		"theme": "default",
		"output": "png",
		"fallback_text": "帮助菜单暂时不可用。",
		"data": {
			"title": "帮助菜单"
		}
	}`))
	if err != nil {
		t.Fatalf("parseRenderImageAction: %v", err)
	}
	if action.Kind != "render.image" || action.RenderTemplate != "help.menu" || action.RenderOutput != "png" {
		t.Fatalf("unexpected render.image action: %#v", action)
	}
	if action.RenderData["title"] != "帮助菜单" {
		t.Fatalf("unexpected render.image data: %#v", action.RenderData)
	}
}

func TestParseRenderImageActionKeepsOmittedOutputEmpty(t *testing.T) {
	t.Parallel()

	action, err := runtimeaction.ParseLocalAction("render.image", json.RawMessage(`{
		"template": "help.menu",
		"data": {
			"title": "帮助菜单"
		}
	}`))
	if err != nil {
		t.Fatalf("parseRenderImageAction: %v", err)
	}
	if action.RenderOutput != "" {
		t.Fatalf("RenderOutput = %q, want empty", action.RenderOutput)
	}
}

func TestClassifyProtocolReadErrorTreatsExitedProcessAsInternalError(t *testing.T) {
	t.Parallel()

	handle := runtimeprocess.NewHandle(nil, nil, nil, runtimeprocess.Spec{})
	handle.SetExit(nil)

	err := classifyProtocolReadError(handle, os.ErrClosed, "plugin exited before init_ack", "read plugin init response")
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestClassifyProtocolReadErrorKeepsProtocolViolationForLiveProcess(t *testing.T) {
	t.Parallel()

	handle := runtimeprocess.NewHandle(nil, nil, nil, runtimeprocess.Spec{})

	err := classifyProtocolReadError(handle, errors.New("short read"), "plugin exited before init_ack", "read plugin init response")
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerDeliverEventFailsWhenRuntimeIsNotRunning(t *testing.T) {
	t.Parallel()

	manager := testManager()

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePlatformInvalidRequest)
}

func TestManagerDeliverEventTimeoutStopsRuntime(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithEventTimeout(t, "event-timeout", "", 80*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginEventTimeout)

	waitForRuntimeState(t, manager, StateStopped)

	_, err = manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePlatformInvalidRequest)
}

func TestManagerPingReturnsPong(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "ping-pong", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	if err := manager.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerPingFailsWhenRuntimeIsNotRunning(t *testing.T) {
	t.Parallel()

	manager := testManager()

	err := manager.Ping(context.Background())
	assertRuntimeErrorCode(t, err, codePlatformInvalidRequest)
}

func TestManagerPingTimeoutStopsRuntime(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithEventTimeout(t, "ping-timeout", "", 80*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	err := manager.Ping(context.Background())
	assertRuntimeErrorCode(t, err, codePluginEventTimeout)

	waitForRuntimeState(t, manager, StateStopped)
}

func TestManagerPingRejectsProtocolViolation(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "ping-wrong-type", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	err := manager.Ping(context.Background())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerStopIgnoresPluginThatAlreadyExited(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "exit-after-ready", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	waitForRuntimeState(t, manager, StateStopped)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime after plugin exit: %v", err)
	}
}

func helperSpec(t *testing.T, scenario string, recordPath string) runtimespec.Spec {
	t.Helper()

	return helperSpecWithTimings(
		t,
		scenario,
		recordPath,
		2*time.Second,
		4*time.Second,
		400*time.Millisecond,
	)
}

func helperSpecWithEventTimeout(t *testing.T, scenario string, recordPath string, eventTimeout time.Duration) runtimespec.Spec {
	t.Helper()

	spec := helperSpec(t, scenario, recordPath)
	spec.EventTimeout = eventTimeout
	return spec
}

func helperSpecWithConcurrency(t *testing.T, scenario string, recordPath string, concurrency int) runtimespec.Spec {
	t.Helper()

	spec := helperSpec(t, scenario, recordPath)
	if concurrency < 1 {
		concurrency = 1
	}
	spec.EffectiveConcurrency = concurrency
	return spec
}

func helperSpecWithTimings(t *testing.T, scenario string, recordPath string, initTimeout time.Duration, initMaxTotal time.Duration, shutdownGrace time.Duration) runtimespec.Spec {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	env := append([]string(nil), os.Environ()...)
	env = append(env, "RAYLEABOT_RUNTIME_HELPER=1")
	env = append(env, "RAYLEABOT_RUNTIME_SCENARIO="+scenario)
	if recordPath != "" {
		env = append(env, "RAYLEABOT_RUNTIME_RECORD="+recordPath)
	}

	return runtimespec.Spec{
		PluginID:             "helper-plugin",
		Runtime:              "test",
		Command:              executable,
		Args:                 []string{"-test.run=TestHelperProcessRuntime", "--"},
		Env:                  env,
		WorkDir:              t.TempDir(),
		EntryPath:            "helper",
		InitTimeout:          runtimeTestDuration(initTimeout),
		InitMaxTotal:         runtimeTestDuration(initMaxTotal),
		EventTimeout:         runtimeTestDuration(300 * time.Millisecond),
		ShutdownGrace:        runtimeTestDuration(shutdownGrace),
		EffectiveConcurrency: 1,
	}
}

func testInitPayload() runtimespec.InitPayload {
	return runtimespec.InitPayload{
		Bot: runtimespec.BotInfo{
			ID:       "bot-1",
			Nickname: "RayleaBot",
		},
		Capabilities:    []string{"event.subscribe"},
		SuperAdmins:     []string{"9001", "9002"},
		CommandPrefixes: []string{"!", "/"},
	}
}

func testRuntimeEvent() runtimeprotocol.Event {
	return runtimeprotocol.Event{
		EventID:        "evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message.group",
		Timestamp:      time.Unix(1_700_000_200, 0).Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID: "3001",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "group",
			ID:   "2001",
		},
		Message: &runtimeprotocol.EventMessage{
			PlainText: "hello from adapter bridge",
		},
	}
}

func testRuntimeEventWithTarget(targetID string) runtimeprotocol.Event {
	event := testRuntimeEvent()
	event.EventID = "evt-" + targetID
	event.Target = &runtimeprotocol.EventTarget{
		Type: "group",
		ID:   targetID,
	}
	return event
}

func TestManagerCrashInvokesCrashCallback(t *testing.T) {
	t.Parallel()

	crashCh := make(chan struct{}, 1)
	var gotPluginID string
	var gotCrashCount int

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			return "req_test"
		},
	}, Options{
		OnCrash: func(pluginID string, crashCount int, lastErrorCode string) {
			gotPluginID = pluginID
			gotCrashCount = crashCount
			crashCh <- struct{}{}
		},
	})

	spec := helperSpec(t, "crash-after-ready", "")
	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	select {
	case <-crashCh:
	case <-time.After(2 * time.Second):
		t.Fatal("crash callback was not invoked within timeout")
	}

	if gotPluginID != spec.PluginID {
		t.Errorf("crash callback plugin_id: got %q want %q", gotPluginID, spec.PluginID)
	}
	if gotCrashCount != 1 {
		t.Errorf("crash callback crash_count: got %d want 1", gotCrashCount)
	}

	snapshot := manager.Snapshot()
	if snapshot.State != StateCrashed {
		t.Errorf("runtime state after crash: got %q want %q", snapshot.State, StateCrashed)
	}
	if snapshot.CrashCount != 1 {
		t.Errorf("crash count in snapshot: got %d want 1", snapshot.CrashCount)
	}
}

func TestManagerCrashCountIncrementsAcrossMultipleCrashes(t *testing.T) {
	t.Parallel()

	crashCh := make(chan int, 5)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			return "req_test"
		},
	}, Options{
		OnCrash: func(_ string, crashCount int, _ string) {
			crashCh <- crashCount
		},
	})

	for i := 1; i <= 3; i++ {
		spec := helperSpec(t, "crash-after-ready", "")
		// Reset to stopped so Start() can proceed
		manager.SetStopped()

		if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
			t.Fatalf("start runtime (iteration %d): %v", i, err)
		}

		select {
		case count := <-crashCh:
			if count != i {
				t.Errorf("iteration %d: crash_count = %d, want %d", i, count, i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: crash callback not invoked", i)
		}
	}
}

func TestManagerResetCrashCount(t *testing.T) {
	t.Parallel()

	manager := testManager()
	manager.mu.Lock()
	manager.snap.CrashCount = 3
	manager.mu.Unlock()

	manager.ResetCrashCount()

	snapshot := manager.Snapshot()
	if snapshot.CrashCount != 0 {
		t.Errorf("crash count after reset: got %d want 0", snapshot.CrashCount)
	}
}

func TestManagerSetBackoffState(t *testing.T) {
	t.Parallel()

	manager := testManager()
	nextRetry := time.Now().Add(10 * time.Second)

	manager.SetBackoffState(nextRetry)

	snapshot := manager.Snapshot()
	if snapshot.State != StateBackoff {
		t.Errorf("state after SetBackoffState: got %q want %q", snapshot.State, StateBackoff)
	}
	if snapshot.NextRetryAt == nil {
		t.Fatal("NextRetryAt should not be nil after SetBackoffState")
	}
}

func TestManagerSetDeadLetterState(t *testing.T) {
	t.Parallel()

	manager := testManager()

	manager.SetDeadLetterState()

	snapshot := manager.Snapshot()
	if snapshot.State != StateDeadLetter {
		t.Errorf("state after SetDeadLetterState: got %q want %q", snapshot.State, StateDeadLetter)
	}
	if snapshot.EnteredDeadLetterAt == nil {
		t.Fatal("EnteredDeadLetterAt was not recorded")
	}
	expected := time.Unix(1_700_000_000, 0).UTC()
	if !snapshot.EnteredDeadLetterAt.Equal(expected) {
		t.Errorf("EnteredDeadLetterAt: got %s want %s", snapshot.EnteredDeadLetterAt, expected)
	}

	manager.ResetCrashCount()
	if manager.Snapshot().EnteredDeadLetterAt != nil {
		t.Error("ResetCrashCount should clear EnteredDeadLetterAt")
	}

	manager.SetDeadLetterState()
	if manager.Snapshot().EnteredDeadLetterAt == nil {
		t.Fatal("EnteredDeadLetterAt should be re-recorded on second entry")
	}
	manager.SetStopped()
	if manager.Snapshot().EnteredDeadLetterAt != nil {
		t.Error("SetStopped should clear EnteredDeadLetterAt")
	}
}

func testManager() *Manager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			return "req_test"
		},
	}, Options{})
}

func testManagerWithOptions(options Options) *Manager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			return "req_test"
		},
	}, options)
}

func testManagerWithRequestIDs(options Options) *Manager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var (
		mu      sync.Mutex
		counter int
	)
	return newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			mu.Lock()
			defer mu.Unlock()
			counter++
			return fmt.Sprintf("req_test_%d", counter)
		},
	}, options)
}

func assertRuntimeErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected runtime error %q, got nil", want)
	}

	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *runtime.Error, got %T", err)
	}
	if runtimeErr.Code != want {
		t.Fatalf("unexpected runtime error code: got %q want %q", runtimeErr.Code, want)
	}
}

func assertActionErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected action error %q, got nil", want)
	}

	var actionErr *runtimeaction.Error
	if !errors.As(err, &actionErr) {
		t.Fatalf("expected *action.Error, got %T", err)
	}
	if actionErr.Code != want {
		t.Fatalf("unexpected action error code: got %q want %q", actionErr.Code, want)
	}
}

func recordedFrames(t *testing.T, path string) []map[string]any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recorded frames: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	frames := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("decode recorded frame %q: %v", line, err)
		}
		frames = append(frames, frame)
	}

	return frames
}

func waitForRuntimeState(t *testing.T, manager *Manager, want State) {
	t.Helper()

	deadline := time.Now().Add(runtimeTestDuration(500 * time.Millisecond))
	for time.Now().Before(deadline) {
		if snapshot := manager.Snapshot(); snapshot.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("runtime did not reach state %q; last snapshot: %+v", want, manager.Snapshot())
}

func runtimeTestDuration(base time.Duration) time.Duration {
	if testing.CoverMode() != "" || testRaceEnabled {
		return base * 3
	}
	return base
}
