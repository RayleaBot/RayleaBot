package bridge

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func TestBridgeQueuesSupportedEventToDispatcher(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "weather",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}

	if len(fakeDispatcher.events) != 1 {
		t.Fatalf("unexpected dispatcher delivery count: got %d want 1", len(fakeDispatcher.events))
	}
	delivered := fakeDispatcher.events[0]
	if delivered.EventType != "message.group" {
		t.Fatalf("unexpected delivered event type: got %q want %q", delivered.EventType, "message.group")
	}
	if delivered.Message == nil || delivered.Message.PlainText != "hello bridge" {
		t.Fatalf("unexpected delivered message: %#v", delivered.Message)
	}
	if fakeDispatcher.commands[0] != "" {
		t.Fatalf("unexpected command routing hint: got %q want empty", fakeDispatcher.commands[0])
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.AcceptedCount != 1 || snapshot.DeliveredCount != 1 || snapshot.ResultCount != 1 {
		t.Fatalf("unexpected bridge counters: %+v", snapshot)
	}
}

func TestBridgeReturnsErrorWhenDispatcherCannotQueueAnyTarget(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{
			{PluginID: "weather", Outcome: dispatch.OutcomeDropped},
			{PluginID: "echo", Outcome: dispatch.OutcomeError, ErrorCode: "platform.invalid_request"},
		},
	}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeError)
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.ErrorCount != 1 {
		t.Fatalf("unexpected error count: %+v", snapshot)
	}
	if snapshot.LastErrorCode != "plugin.internal_error" {
		t.Fatalf("unexpected last error code: got %q want %q", snapshot.LastErrorCode, "plugin.internal_error")
	}
}

func TestBridgeIgnoresUnsupportedAdapterEventShape(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{deliverable: true}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:      "onebot11.unsupported",
		EventType: "message.segmented",
	})
	if outcome != OutcomeIgnored {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeIgnored)
	}
	if len(fakeDispatcher.events) != 0 {
		t.Fatalf("unsupported event should not reach dispatcher")
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.IgnoredCount != 1 {
		t.Fatalf("unexpected ignored count: %+v", snapshot)
	}
}

func TestBridgeRejectsEventWhenNoDeliverableRuntimeExists(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{deliverable: false}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeRejected {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeRejected)
	}
	if len(fakeDispatcher.events) != 0 {
		t.Fatalf("dispatcher should not receive event when nothing is deliverable")
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.RejectedCount != 1 {
		t.Fatalf("unexpected rejected count: %+v", snapshot)
	}
}

func TestBridgeRejectsEventWhenNoTargetAccepts(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{deliverable: true}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent())
	if outcome != OutcomeRejected {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeRejected)
	}
	if len(fakeDispatcher.events) != 1 {
		t.Fatalf("dispatcher should inspect the event once, got %d", len(fakeDispatcher.events))
	}
}

func TestBridgeDeliversFriendRequestEvent(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "friend-handler",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindRequest,
		EventID:          "onebot11-request-friend-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "request.friend",
		Timestamp:        time.Unix(1_700_000_456, 0).Unix(),
		ConversationType: "private",
		ConversationID:   "2001",
		SenderID:         "2001",
		PayloadFields: map[string]any{
			"flag":    "friend-flag-1",
			"comment": "请通过好友申请",
		},
	})
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}
	if len(fakeDispatcher.events) != 1 {
		t.Fatalf("unexpected dispatcher delivery count: got %d want 1", len(fakeDispatcher.events))
	}
	if fakeDispatcher.events[0].EventType != "request.friend" {
		t.Fatalf("unexpected delivered event type: %q", fakeDispatcher.events[0].EventType)
	}
	if got := fakeDispatcher.events[0].PayloadFields["flag"]; got != "friend-flag-1" {
		t.Fatalf("unexpected payload flag: %#v", got)
	}
}

func TestBridgeDeliversMessageSentEvent(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "self-log",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessageSent,
		EventID:          "onebot11-message-sent-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message_sent.group",
		Timestamp:        time.Unix(1_700_000_456, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		MessageID:        "1001",
		PlainText:        "hello self",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message_sent",
				"message_type":   "group",
				"group_id":       "2001",
				"user_id":        "3001",
				"time":           int64(1_700_000_456),
				"message_id":     "1001",
				"real_id":        "1001",
				"message_seq":    "1001",
				"raw_message":    "hello self",
				"message_format": "array",
				"font":           14,
				"sender": map[string]any{
					"nickname": "Alice",
					"role":     "owner",
				},
			},
		},
	})
	if outcome != OutcomeDelivered {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeDelivered)
	}
	if len(fakeDispatcher.events) != 1 {
		t.Fatalf("unexpected dispatcher delivery count: got %d want 1", len(fakeDispatcher.events))
	}
	if fakeDispatcher.events[0].EventType != "message_sent.group" {
		t.Fatalf("unexpected delivered event type: %q", fakeDispatcher.events[0].EventType)
	}
	onebot, ok := fakeDispatcher.events[0].PayloadFields["onebot"].(map[string]any)
	if !ok {
		t.Fatalf("missing onebot payload: %#v", fakeDispatcher.events[0].PayloadFields)
	}
	if got := onebot["post_type"]; got != "message_sent" {
		t.Fatalf("unexpected post_type: %#v", got)
	}
}

func TestBridgeEventSummaryFormatsGroupMessageContext(t *testing.T) {
	t.Parallel()

	summary := bridgeEventSummary("queued for dispatcher", adapter.NormalizedEvent{
		BotID:            "1145141919",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "553855023",
		SenderID:         "1358252269",
		TargetName:       "终末地摸鱼群",
		PlainText:        "除了战猎这种抓不到加费就完全没法打的角色",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"sender": map[string]any{
					"nickname": "没错，是魔法！",
					"card":     "群星怒",
					"title":    "管理员",
				},
			},
		},
	})

	if summary != "1145141919: [终末地摸鱼群(553855023)][管理员]群星怒/没错，是魔法！(1358252269): 除了战猎这种抓不到加费就完全没法打的角色" {
		t.Fatalf("unexpected group summary: %#v", summary)
	}
}

func TestBridgeEventSummaryFormatsPrivateMessageContext(t *testing.T) {
	t.Parallel()

	summary := bridgeEventSummary("queued for dispatcher", adapter.NormalizedEvent{
		BotID:          "1145141919",
		SourceProtocol: "onebot11",
		EventType:      "message.private",
		SenderID:       "3599026669",
		PlainText:      "你好",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"sender": map[string]any{
					"nickname": "乔温迪乔斯达",
				},
			},
		},
	})

	if summary != "1145141919: 乔温迪乔斯达(3599026669): 你好" {
		t.Fatalf("unexpected private summary: %#v", summary)
	}
}

func TestBridgeEventLogAttrsIncludeBotIDAndGroupName(t *testing.T) {
	t.Parallel()

	attrs := bridgeEventLogAttrs(adapter.NormalizedEvent{
		BotID:            "1145141919",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "553855023",
		SenderID:         "1358252269",
		TargetName:       "终末地摸鱼群",
		PlainText:        "hello bridge",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"self_id": "1145141919",
				"sender": map[string]any{
					"nickname": "Alice",
				},
			},
		},
	})

	attrMap := make(map[string]any, len(attrs)/2)
	for index := 0; index+1 < len(attrs); index += 2 {
		key, _ := attrs[index].(string)
		attrMap[key] = attrs[index+1]
	}

	if attrMap["self_id"] != "1145141919" {
		t.Fatalf("unexpected self_id attr: %#v", attrMap["self_id"])
	}
	if attrMap["group_name"] != "终末地摸鱼群" {
		t.Fatalf("unexpected group_name attr: %#v", attrMap["group_name"])
	}
}

type recordingDispatcher struct {
	deliverable bool
	results     []dispatch.DeliveryResult
	events      []runtime.Event
	commands    []string
}

func (r *recordingDispatcher) HasDeliverablePlugins() bool {
	return r.deliverable
}

func (r *recordingDispatcher) Dispatch(_ context.Context, event runtime.Event, commandName string) []dispatch.DeliveryResult {
	r.events = append(r.events, event)
	r.commands = append(r.commands, commandName)
	return append([]dispatch.DeliveryResult(nil), r.results...)
}

func testBridge(dispatcher dispatcherClient) *Bridge {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, dispatcher)
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
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":      "message",
				"message_type":   "group",
				"group_id":       "2001",
				"user_id":        "3001",
				"time":           int64(1_700_000_123),
				"message_id":     "1001",
				"real_id":        "1001",
				"message_seq":    "1001",
				"raw_message":    "hello bridge",
				"message_format": "array",
				"font":           14,
				"sender": map[string]any{
					"nickname": "Alice",
					"card":     "管理员",
					"role":     "admin",
				},
			},
		},
	}
}
