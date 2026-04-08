package bridge

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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
					MessageSegments: []runtime.ActionSegment{{
						Type: "text",
						Data: map[string]any{"text": "hello bridge"},
					}},
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
	if fakeSender.actions[0].TargetType != "group" || fakeSender.actions[0].TargetID != "2001" {
		t.Fatalf("unexpected outbound action payload: %#v", fakeSender.actions[0])
	}
	if len(fakeSender.actions[0].Segments) != 1 || fakeSender.actions[0].Segments[0].Type != "text" || fakeSender.actions[0].Segments[0].Data["text"] != "hello bridge" {
		t.Fatalf("unexpected outbound action segments: %#v", fakeSender.actions[0])
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
					MessageSegments: []runtime.ActionSegment{{
						Type: "text",
						Data: map[string]any{"text": "hello bridge"},
					}},
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

func TestBridgeDeliversMessageReplyAction(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{
		sendResult: adapter.SendMessageResult{MessageID: "9002"},
	}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_reply",
				Action: &runtime.Action{
					Kind:           "message.reply",
					ReplyToEventID: "onebot11-message-12345",
					MessageSegments: []runtime.ActionSegment{{
						Type: "text",
						Data: map[string]any{"text": "今日天气：晴"},
					}},
				},
			}, nil
		},
	}
	eventBridge := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeRuntime,
		fakeSender,
		stubReplyTargetResolver{
			"onebot11-message-12345": {
				MessageID:  "98765",
				TargetType: "group",
				TargetID:   "2001",
			},
		},
	)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}
	if len(fakeSender.replyActions) != 1 {
		t.Fatalf("expected one reply action, got %d", len(fakeSender.replyActions))
	}
	if fakeSender.replyActions[0].ReplyToMessageID != "98765" {
		t.Fatalf("unexpected reply action payload: %#v", fakeSender.replyActions[0])
	}
	if len(fakeSender.replyActions[0].Segments) != 1 || fakeSender.replyActions[0].Segments[0].Type != "text" || fakeSender.replyActions[0].Segments[0].Data["text"] != "今日天气：晴" {
		t.Fatalf("unexpected reply action segments: %#v", fakeSender.replyActions[0])
	}
	if len(fakeSender.actions) != 0 {
		t.Fatalf("message.reply should not call SendMessage, got %d calls", len(fakeSender.actions))
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
					Kind: "message.broadcast",
				},
			}, nil
		},
	}
	eventBridge := testBridge(fakeRuntime, fakeSender)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeError)
	}
	if len(fakeSender.actions) != 0 || len(fakeSender.replyActions) != 0 {
		t.Fatalf("unsupported action kind should not reach adapter sender")
	}
}

func TestBridgeFallsBackToSendWhenRichReplyTargetIsMissing(t *testing.T) {
	t.Parallel()

	fakeSender := &fakeActionSender{
		sendResult: adapter.SendMessageResult{MessageID: "9003"},
		replyErr: &adapter.Error{
			Code:    "adapter.reply_target_missing",
			Message: "reply target missing",
		},
	}
	fakeRuntime := &fakeRuntimeClient{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverFunc: func(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
			return runtime.Delivery{
				RequestID: "req_evt_reply_fallback",
				Action: &runtime.Action{
					Kind:                    "message.reply",
					ReplyToEventID:          "onebot11-message-12345",
					FallbackToSendIfMissing: true,
					MessageSegments: []runtime.ActionSegment{{
						Type: "text",
						Data: map[string]any{"text": "rich fallback body"},
					}},
				},
			}, nil
		},
	}

	eventBridge := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeRuntime,
		fakeSender,
		stubReplyTargetResolver{
			"onebot11-message-12345": {
				MessageID:  "98765",
				TargetType: "group",
				TargetID:   "2001",
			},
		},
	)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}
	if len(fakeSender.replyActions) != 1 {
		t.Fatalf("expected one reply attempt, got %d", len(fakeSender.replyActions))
	}
	if len(fakeSender.actions) != 1 {
		t.Fatalf("expected one fallback send, got %d", len(fakeSender.actions))
	}
	if fakeSender.actions[0].TargetType != "group" || fakeSender.actions[0].TargetID != "2001" {
		t.Fatalf("unexpected fallback send target: %#v", fakeSender.actions[0])
	}
	if len(fakeSender.actions[0].Segments) != 1 || fakeSender.actions[0].Segments[0].Type != "text" {
		t.Fatalf("unexpected fallback segments: %#v", fakeSender.actions[0].Segments)
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
	actions      []adapter.OutboundMessageSend
	replyActions []adapter.OutboundMessageReply
	sendResult   adapter.SendMessageResult
	sendErr      error
	replyErr     error
}

func (f *fakeActionSender) SendMessage(ctx context.Context, action adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	f.actions = append(f.actions, action)
	if f.sendErr != nil {
		return adapter.SendMessageResult{}, f.sendErr
	}
	return f.sendResult, nil
}

func (f *fakeActionSender) SendReply(ctx context.Context, action adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	f.replyActions = append(f.replyActions, action)
	if f.replyErr != nil {
		return adapter.SendMessageResult{}, f.replyErr
	}
	if f.sendErr != nil {
		return adapter.SendMessageResult{}, f.sendErr
	}
	return f.sendResult, nil
}

func testBridge(runtimeClient runtimeClient, sender *fakeActionSender) *Bridge {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, runtimeClient, sender, nil)
}

type stubReplyTargetResolver map[string]struct {
	MessageID  string
	TargetType string
	TargetID   string
}

func (r stubReplyTargetResolver) ResolveReplyTarget(eventID string) (outbound.ReplyTarget, bool) {
	target, ok := r[eventID]
	if !ok {
		return outbound.ReplyTarget{}, false
	}
	return outbound.ReplyTarget{
		MessageID:  target.MessageID,
		TargetType: target.TargetType,
		TargetID:   target.TargetID,
	}, true
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
