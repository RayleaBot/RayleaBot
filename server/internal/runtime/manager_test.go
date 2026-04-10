package runtime

import (
	"bufio"
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

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartAllowsInitProgressBeforeReady(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "progress-then-ready", "", 500*time.Millisecond, 2*time.Second, 400*time.Millisecond)

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
	spec := helperSpecWithTimings(t, "early-exit", "", time.Second, 3*time.Second, 400*time.Millisecond)

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestManagerStartSucceedsWithLargeStderrOutput(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "stderr-noise", "", time.Second, 3*time.Second, 400*time.Millisecond)

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

	frame := buildEventFrame(Event{
		EventID:        "evt-onebot-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message_sent.group",
		Timestamp:      1_729_679_125,
		Actor: &EventActor{
			ID:       "721011692",
			Nickname: "--",
			Role:     "owner",
		},
		Target: &EventTarget{
			Type: "group",
			ID:   "860105388",
		},
		Message: &EventMessage{
			PlainText: "您好",
			Segments: []EventSegment{{
				Type: "text",
				Data: map[string]any{"text": "您好"},
			}},
		},
		MessageID: "966671988",
		PayloadFields: map[string]any{
			"sub_type": "normal",
			"onebot": map[string]any{
				"post_type":      "message_sent",
				"message_type":   "group",
				"self_id":        "721011692",
				"user_id":        "721011692",
				"group_id":       "860105388",
				"time":           int64(1_729_679_125),
				"message_id":     "966671988",
				"real_id":        "966671988",
				"message_seq":    "966671988",
				"raw_message":    "您好",
				"font":           14,
				"message_format": "array",
				"sender": map[string]any{
					"user_id":  "721011692",
					"nickname": "--",
					"role":     "owner",
				},
			},
		},
	}, "echo", "req_evt_onebot", 1_729_679_126)

	if frame.Event.Payload == nil || frame.Event.Payload.OneBot == nil {
		t.Fatalf("expected onebot payload, got %#v", frame.Event.Payload)
	}
	if frame.Event.Payload.MessageID != "966671988" {
		t.Fatalf("unexpected payload message_id: %#v", frame.Event.Payload.MessageID)
	}
	if frame.Event.Payload.OneBot.PostType != "message_sent" {
		t.Fatalf("unexpected onebot post_type: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.GroupID != "860105388" {
		t.Fatalf("unexpected onebot group_id: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.Time != 1_729_679_125 {
		t.Fatalf("unexpected onebot time: %#v", frame.Event.Payload.OneBot)
	}
	if frame.Event.Payload.OneBot.Sender == nil || frame.Event.Payload.OneBot.Sender.Nickname != "--" {
		t.Fatalf("unexpected onebot sender: %#v", frame.Event.Payload.OneBot.Sender)
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

func TestManagerDeliverEventRejectsLegacyMessageReplyAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-action-message-reply", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerDeliverEventRejectsRemovedMessageSendImageAction(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "event-action-message-send-image", "")

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	_, err := manager.DeliverEvent(context.Background(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)

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

	_, err := parseMessageSendAction(json.RawMessage(`{
		"target_type": "group",
		"target_id": "2001",
		"text": "removed text payload"
	}`))
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerDeliverEventProcessesLocalActionsBeforeTerminalResult(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		actions []Action
	)
	manager := testManagerWithOptions(Options{
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action Action) (map[string]any, error) {
			mu.Lock()
			actions = append(actions, action)
			mu.Unlock()

			if pluginID != "helper-plugin" {
				t.Fatalf("pluginID = %q, want helper-plugin", pluginID)
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
		ExecuteLocalAction: func(_ context.Context, _ string, _ string, action Action) (map[string]any, error) {
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
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action Action) (map[string]any, error) {
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
		ExecuteLocalAction: func(context.Context, string, string, Action) (map[string]any, error) {
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
		ExecuteLocalAction: func(context.Context, string, string, Action) (map[string]any, error) {
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
		ExecuteLocalAction: func(_ context.Context, pluginID string, requestID string, action Action) (map[string]any, error) {
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

	action, err := parseStorageFileAction(json.RawMessage(`{
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

	_, err := parseHTTPRequestAction(json.RawMessage(`{
		"method": "GET",
		"url": "https://api.example.test/v1/data",
		"body_text": "denied"
	}`))
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestParseConfigReadAction(t *testing.T) {
	t.Parallel()

	action, err := parseConfigReadAction(json.RawMessage(`{
		"keys": ["default_city", "unit"]
	}`))
	if err != nil {
		t.Fatalf("parseConfigReadAction: %v", err)
	}
	if action.Kind != "config.read" || len(action.ConfigKeys) != 2 {
		t.Fatalf("unexpected config.read action: %#v", action)
	}
}

func TestParseConfigWriteAction(t *testing.T) {
	t.Parallel()

	action, err := parseConfigWriteAction(json.RawMessage(`{
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

func TestParseSchedulerCreateAction(t *testing.T) {
	t.Parallel()

	action, err := parseSchedulerCreateAction(json.RawMessage(`{
		"task_id": "daily_report",
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
}

func TestParseEventExposeWebhookAction(t *testing.T) {
	t.Parallel()

	action, err := parseEventExposeWebhookAction(json.RawMessage(`{
		"route": "github",
		"methods": ["POST"],
		"auth_strategy": "hmac_sha256",
		"header": "X-Hub-Signature-256",
		"signature_prefix": "sha256=",
		"secret_ref": "webhook.github.secret",
		"source_ips": ["192.0.2.0/24"]
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
}

func TestParseRenderImageAction(t *testing.T) {
	t.Parallel()

	action, err := parseRenderImageAction(json.RawMessage(`{
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

func TestClassifyProtocolReadErrorTreatsExitedProcessAsInternalError(t *testing.T) {
	t.Parallel()

	handle := &processHandle{done: make(chan struct{})}
	handle.setExit(nil)

	err := classifyProtocolReadError(handle, os.ErrClosed, "plugin exited before init_ack", "read plugin init response")
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestClassifyProtocolReadErrorKeepsProtocolViolationForLiveProcess(t *testing.T) {
	t.Parallel()

	handle := &processHandle{done: make(chan struct{})}

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
	spec := helperSpecWithTimings(t, "exit-after-ready", "", 300*time.Millisecond, time.Second, 400*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	waitForRuntimeState(t, manager, StateStopped)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime after plugin exit: %v", err)
	}
}

func TestHelperProcessRuntime(t *testing.T) {
	if os.Getenv("RAYLEABOT_RUNTIME_HELPER") != "1" {
		return
	}

	scenario := os.Getenv("RAYLEABOT_RUNTIME_SCENARIO")
	recordPath := os.Getenv("RAYLEABOT_RUNTIME_RECORD")
	scanner := bufio.NewScanner(os.Stdin)

	switch scenario {
	case "ping-pong":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			switch frame["type"] {
			case "ping":
				writeHelperFrame(map[string]any{
					"protocol_version": "1",
					"type":             "pong",
					"timestamp":        time.Now().Unix(),
					"plugin_id":        frame["plugin_id"],
					"request_id":       frame["request_id"],
				})
			case "shutdown":
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "ping-timeout":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		// receive ping but never respond — triggers timeout
		if scanner.Scan() {
			time.Sleep(500 * time.Millisecond)
		}
		os.Exit(0)
	case "ping-wrong-type":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			// respond with wrong type instead of pong
			writeHelperFrame(map[string]any{
				"protocol_version": "1",
				"type":             "result",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        frame["plugin_id"],
				"request_id":       frame["request_id"],
				"status":           "success",
				"data":             map[string]any{},
			})
		}
		for scanner.Scan() {
		}
		os.Exit(0)
	case "early-exit":
		if !scanner.Scan() {
			os.Exit(2)
		}
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "event-action-message-send":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.send",
			"data": map[string]any{
				"target_type": "group",
				"target_id":   "2001",
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "text",
							"data": map[string]any{"text": "hello from plugin"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-error":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "error",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"code":             "plugin.not_handled",
			"message":          "plugin chose not to handle this event",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		recordFrame(recordPath, line)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			recordFrame(recordPath, line)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-reply":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.reply",
			"data": map[string]any{
				removedReplyMessageIDKey(): "98765",
				"text":                     "reply from plugin",
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-send-image":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           removedSendImageActionKind(),
			"data": map[string]any{
				"target_type": "group",
				"target_id":   "2001",
				"file":        "file://cache/image.png",
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-send-segments":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.send",
			"data": map[string]any{
				"target_type": "group",
				"target_id":   "2001",
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "at",
							"data": map[string]any{"user_id": "3001"},
						},
						{
							"type": "text",
							"data": map[string]any{"text": "hello rich runtime"},
						},
						{
							"type": "image",
							"data": map[string]any{"file": "file://cache/weather.png"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-reply-to-event":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.reply",
			"data": map[string]any{
				"reply_to_event_id":           "onebot11-message-12345",
				"fallback_to_send_if_missing": true,
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "text",
							"data": map[string]any{"text": "rich reply body"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-local-actions-then-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_logger_1",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "notice.member_increase received",
				"fields": map[string]any{
					"event_id": eventFrame["request_id"],
				},
			},
		})
		helperExpectFrameType(scanner, "local_logger_1", "result", 5)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_storage_1",
			"action":           "storage.kv",
			"data": map[string]any{
				"operation": "get",
				"key":       "notice:last_join",
			},
		})
		localStorageResult := helperExpectFrameType(scanner, "local_storage_1", "result", 6)
		data, _ := localStorageResult["data"].(map[string]any)
		exists, _ := data["exists"].(bool)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled":        true,
				"storage_exists": exists,
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	case "event-local-action-error-then-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_logger_2",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "warn",
				"message": "attempt denied",
			},
		})
		localError := helperExpectFrameType(scanner, "local_logger_2", "error", 5)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled":          true,
				"local_error_code": localError["code"],
			},
		})
		helperConsumeShutdown(scanner, 6)
		os.Exit(0)
	case "event-local-action-missing-parent-request-id":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_logger_missing_parent",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "missing parent_request_id should fail",
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "event-concurrent-local-actions-then-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		parentRequestID, _ := eventFrame["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_logger_3",
			"parent_request_id": parentRequestID,
			"action":            "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "first concurrent local action",
			},
		})
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_storage_3",
			"parent_request_id": parentRequestID,
			"action":            "storage.kv",
			"data": map[string]any{
				"operation": "get",
				"key":       "concurrent:key",
			},
		})
		firstResponse := helperReadFrame(scanner, 5)
		secondResponse := helperReadFrame(scanner, 6)
		seen := map[string]bool{}
		for _, frame := range []map[string]any{firstResponse, secondResponse} {
			requestID, _ := frame["request_id"].(string)
			frameType, _ := frame["type"].(string)
			if frameType != "result" {
				os.Exit(205)
			}
			seen[requestID] = true
		}
		if !seen["local_logger_3"] || !seen["local_storage_3"] {
			os.Exit(206)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       parentRequestID,
			"status":           "success",
			"data": map[string]any{
				"logger_started":  seen["local_logger_3"],
				"storage_started": seen["local_storage_3"],
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	case "event-local-action-early-terminal-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		parentRequestID, _ := eventFrame["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_logger_pending",
			"parent_request_id": parentRequestID,
			"action":            "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "pending local action",
			},
		})
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       parentRequestID,
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "event-local-action-same-request-id":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "this should fail",
			},
		})
		os.Exit(0)
	case "event-concurrent-slow-local-action-does-not-block-other-session":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		firstEvent := helperReadFrame(scanner, 4)
		firstRequestID, _ := firstEvent["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         firstEvent["plugin_id"],
			"request_id":        "slow_http_1",
			"parent_request_id": firstRequestID,
			"action":            "http.request",
			"data": map[string]any{
				"method": "GET",
				"url":    "https://example.com/slow",
			},
		})
		secondEvent := helperReadFrame(scanner, 5)
		secondRequestID, _ := secondEvent["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        secondEvent["plugin_id"],
			"request_id":       secondRequestID,
			"status":           "success",
			"data": map[string]any{
				"session": "fast",
			},
		})
		slowResponse := helperExpectFrameType(scanner, "slow_http_1", "result", 6)
		if data, ok := slowResponse["data"].(map[string]any); !ok || data["status_code"] != float64(200) {
			os.Exit(206)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        firstEvent["plugin_id"],
			"request_id":       firstRequestID,
			"status":           "success",
			"data": map[string]any{
				"session": "slow",
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	case "event-unsupported-action":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.broadcast",
			"data": map[string]any{
				"text": "out of scope",
			},
		})
		os.Exit(0)
	case "event-timeout":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		time.Sleep(500 * time.Millisecond)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       "req_test",
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "exit-after-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "progress-forever":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			writeHelperFrame(map[string]any{
				"protocol_version": "1",
				"type":             "init_progress",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        initFrame["plugin_id"],
				"request_id":       initFrame["request_id"],
				"summary":          "still booting",
			})
			<-ticker.C
		}
	case "progress-then-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		time.Sleep(runtimeTestDuration(120 * time.Millisecond))
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_progress",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"summary":          "warming up",
		})
		time.Sleep(runtimeTestDuration(120 * time.Millisecond))
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "stderr-noise":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		if _, err := fmt.Fprint(os.Stderr, strings.Repeat("stderr-noise", 128*1024)); err != nil {
			os.Exit(9)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "stderr-secret":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		if _, err := fmt.Fprintln(os.Stderr, "token=fixture-only-secret"); err != nil {
			os.Exit(9)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "timeout":
		if scanner.Scan() {
			recordFrame(recordPath, scanner.Bytes())
		}
		time.Sleep(2 * time.Second)
		os.Exit(0)
	case "wrong-type":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var initFrame map[string]any
		_ = json.Unmarshal(line, &initFrame)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "shutdown",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"reason":           "stop",
		})
		// Let the runtime consume the invalid frame before the helper exits.
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "success":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
			"subscriptions":    []string{"message.group", "scheduler.trigger"},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			recordFrame(recordPath, line)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "crash-after-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(1) // non-zero exit = crash
	default:
		os.Exit(5)
	}
}

func helperSpec(t *testing.T, scenario string, recordPath string) Spec {
	t.Helper()

	return helperSpecWithTimings(
		t,
		scenario,
		recordPath,
		300*time.Millisecond,
		time.Second,
		400*time.Millisecond,
	)
}

func helperSpecWithEventTimeout(t *testing.T, scenario string, recordPath string, eventTimeout time.Duration) Spec {
	t.Helper()

	spec := helperSpec(t, scenario, recordPath)
	spec.EventTimeout = eventTimeout
	return spec
}

func helperSpecWithConcurrency(t *testing.T, scenario string, recordPath string, concurrency int) Spec {
	t.Helper()

	spec := helperSpec(t, scenario, recordPath)
	if concurrency < 1 {
		concurrency = 1
	}
	spec.EffectiveConcurrency = concurrency
	return spec
}

func helperSpecWithTimings(t *testing.T, scenario string, recordPath string, initTimeout time.Duration, initMaxTotal time.Duration, shutdownGrace time.Duration) Spec {
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

	return Spec{
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

func testInitPayload() InitPayload {
	return InitPayload{
		Bot: BotInfo{
			ID:       "bot-1",
			Nickname: "RayleaBot",
		},
		Capabilities:    []string{"event.subscribe"},
		CommandPrefixes: []string{"!", "/"},
	}
}

func testRuntimeEvent() Event {
	return Event{
		EventID:        "evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message.group",
		Timestamp:      time.Unix(1_700_000_200, 0).Unix(),
		Actor: &EventActor{
			ID: "3001",
		},
		Target: &EventTarget{
			Type: "group",
			ID:   "2001",
		},
		Message: &EventMessage{
			PlainText: "hello from adapter bridge",
		},
	}
}

func testRuntimeEventWithTarget(targetID string) Event {
	event := testRuntimeEvent()
	event.EventID = "evt-" + targetID
	event.Target = &EventTarget{
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

func recordFrame(path string, line []byte) {
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(7)
	}
}

func writeHelperFrame(frame map[string]any) {
	encoded, err := json.Marshal(frame)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	fmt.Printf("%s\n", encoded)
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

func helperReadFrame(scanner *bufio.Scanner, code int) map[string]any {
	if !scanner.Scan() {
		os.Exit(code)
	}
	line := append([]byte(nil), scanner.Bytes()...)
	var frame map[string]any
	if err := json.Unmarshal(line, &frame); err != nil {
		os.Exit(code + 100)
	}
	return frame
}

func helperExpectFrameType(scanner *bufio.Scanner, requestID string, frameType string, code int) map[string]any {
	frame := helperReadFrame(scanner, code)
	if frame["request_id"] != requestID || frame["type"] != frameType {
		os.Exit(code + 200)
	}
	return frame
}

func helperConsumeShutdown(scanner *bufio.Scanner, code int) {
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var frame map[string]any
		if err := json.Unmarshal(line, &frame); err != nil {
			os.Exit(code + 100)
		}
		if frame["type"] == "shutdown" {
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func removedReplyMessageIDKey() string {
	return strings.Join([]string{"reply", "to", "message", "id"}, "_")
}

func removedSendImageActionKind() string {
	return strings.Join([]string{"message", "send_image"}, ".")
}
