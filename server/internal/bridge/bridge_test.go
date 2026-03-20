package bridge

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/runtime"
)

func TestBridgeDeliversSupportedEventToRunningRuntime(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{
		sendResult: adapter.SendMessageResult{MessageID: "9001"},
	}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_1",
				Action: &runtime.Action{
					Kind:       "message.send",
					TargetType: "group",
					TargetID:   "2001",
					Text:       "hello bridge",
				},
			}, nil
		},
	}
	eventBridge := testBridge(fakeRuntime, fakeSender)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}

	if len(fakeRuntime.events) != 1 {
		t.Fatalf("unexpected runtime delivery count: got %d want 1", len(fakeRuntime.events))
	}
	delivered := fakeRuntime.events[0]
	if delivered.EventType != "message.group" {
		t.Fatalf("unexpected delivered event type: got %q want %q", delivered.EventType, "message.group")
	}
	if len(fakeSender.actions) != 1 {
		t.Fatalf("expected one outbound action, got %d", len(fakeSender.actions))
	}
	if fakeSender.actions[0].TargetType != "group" || fakeSender.actions[0].TargetID != "2001" || fakeSender.actions[0].Text != "hello bridge" {
		t.Fatalf("unexpected outbound action payload: %#v", fakeSender.actions[0])
	}
	if delivered.Message == nil || delivered.Message.PlainText != "hello bridge" {
		t.Fatalf("unexpected delivered message: %#v", delivered.Message)
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.AcceptedCount != 1 || snapshot.DeliveredCount != 1 || snapshot.ResultCount != 1 {
		t.Fatalf("unexpected bridge counters: %+v", snapshot)
	}
}

func TestBridgeReturnsPluginErrorForDeliveredEvent(t *testing.T) {
	t.Parallel()

	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
					RequestID:    "req_evt_2",
					ErrorCode:    "plugin.not_handled",
					ErrorMessage: "plugin chose not to handle this event",
				}, &runtime.Error{
					Code:    "plugin.not_handled",
					Message: "plugin chose not to handle this event",
				}
		},
	}
	eventBridge := testBridge(fakeRuntime, nil)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeError)
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.ErrorCount != 1 {
		t.Fatalf("unexpected error count: %+v", snapshot)
	}
	if snapshot.LastErrorCode != "plugin.not_handled" {
		t.Fatalf("unexpected last error code: got %q want %q", snapshot.LastErrorCode, "plugin.not_handled")
	}
}

func TestBridgeIgnoresUnsupportedAdapterEventShape(t *testing.T) {
	t.Parallel()

	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
	}
	eventBridge := testBridge(fakeRuntime, nil)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:      "onebot11.unsupported",
		EventType: "message.segmented",
	})
	if outcome != OutcomeIgnored {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeIgnored)
	}
	if len(fakeRuntime.events) != 0 {
		t.Fatalf("unsupported event should not reach runtime")
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.IgnoredCount != 1 {
		t.Fatalf("unexpected ignored count: %+v", snapshot)
	}
}

func TestBridgeRejectsEventWhenRuntimeIsNotRunning(t *testing.T) {
	t.Parallel()

	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateStopped},
	}
	eventBridge := testBridge(fakeRuntime, nil)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeRejected {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeRejected)
	}
	if len(fakeRuntime.events) != 0 {
		t.Fatalf("runtime should not receive event when stopped")
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.RejectedCount != 1 {
		t.Fatalf("unexpected rejected count: %+v", snapshot)
	}
}

func TestBridgeAllowsOpaqueResultWithoutOutboundAction(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_3",
				Result: map[string]any{
					"handled": true,
				},
			}, nil
		},
	}
	eventBridge := testBridge(fakeRuntime, fakeSender)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}
	if len(fakeRuntime.events) != 1 {
		t.Fatalf("unexpected runtime delivery count: got %d want 1", len(fakeRuntime.events))
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.ResultCount != 1 || snapshot.ErrorCount != 0 || snapshot.RejectedCount != 0 {
		t.Fatalf("unexpected bridge counters after opaque result: %+v", snapshot)
	}
	if len(fakeSender.actions) != 0 {
		t.Fatalf("opaque result should not trigger outbound action: %#v", fakeSender.actions)
	}
}

func TestBridgeReturnsAdapterErrorForOutboundActionFailure(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{
		sendErr: &adapter.Error{
			Code:    "adapter.send_failed",
			Message: "onebot send_msg failed",
		},
	}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_4",
				Action: &runtime.Action{
					Kind:       "message.send",
					TargetType: "group",
					TargetID:   "2001",
					Text:       "hello bridge",
				},
			}, nil
		},
	}
	eventBridge := testBridge(fakeRuntime, fakeSender)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeError)
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.ErrorCount != 1 || snapshot.LastErrorCode != "adapter.send_failed" {
		t.Fatalf("unexpected bridge error snapshot: %+v", snapshot)
	}
}

func TestBridgeRejectsUnsupportedOutboundActionKind(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_5",
				Action: &runtime.Action{
					Kind:       "message.reply",
					TargetType: "group",
					TargetID:   "2001",
					Text:       "out of scope",
				},
			}, nil
		},
	}
	eventBridge := testBridge(fakeRuntime, fakeSender)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeError)
	}
	if len(fakeSender.actions) != 0 {
		t.Fatalf("unsupported action kind should not reach adapter sender")
	}
}

type fakeRuntimeClient struct {
	snapshot    runtime.Snapshot
	deliverFunc func(context.Context, runtime.Event) (runtime.Delivery, error)
	events      []runtime.Event
}

func (f *fakeRuntimeClient) Snapshot() runtime.Snapshot {
	return f.snapshot
}

func (f *fakeRuntimeClient) DeliverEvent(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
	f.events = append(f.events, event)
	if f.deliverFunc == nil {
		return runtime.Delivery{}, nil
	}
	return f.deliverFunc(ctx, event)
}

type fakeActionSender struct {
	actions    []adapter.OutboundMessageSend
	sendResult adapter.SendMessageResult
	sendErr    error
}

func (f *fakeActionSender) SendMessage(ctx context.Context, action adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	f.actions = append(f.actions, action)
	if f.sendErr != nil {
		return adapter.SendMessageResult{}, f.sendErr
	}
	return f.sendResult, nil
}

func testBridge(runtimeClient runtimeClient, sender actionSender) *Bridge {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, runtimeClient, sender)
}

func supportedAdapterEvent() adapter.NormalizedEvent {
	return adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessageText,
		EventID:          "onebot11-message-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Unix(1_700_000_123, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		PlainText:        "hello bridge",
	}
}
