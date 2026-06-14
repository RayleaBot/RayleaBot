package bridge

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
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

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
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

func TestBridgeIgnoresEventWhenNoDeliverableRuntimeExists(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{deliverable: false}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedGroupRecallNoticeEvent())
	if outcome != OutcomeIgnored {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeIgnored)
	}
	if len(fakeDispatcher.events) != 0 {
		t.Fatalf("dispatcher should not receive event when nothing is deliverable")
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.IgnoredCount != 1 || snapshot.RejectedCount != 0 {
		t.Fatalf("unexpected ignored/rejected counts: %+v", snapshot)
	}
	if snapshot.LastErrorCode != "" || snapshot.LastErrorText != "" {
		t.Fatalf("ignored outcome should clear last error fields: %+v", snapshot)
	}
}

func TestBridgeIgnoresEventWhenNoTargetAccepts(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{deliverable: true}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedGroupRecallNoticeEvent())
	if outcome != OutcomeIgnored {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeIgnored)
	}
	if len(fakeDispatcher.events) != 1 {
		t.Fatalf("dispatcher should inspect the event once, got %d", len(fakeDispatcher.events))
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.IgnoredCount != 1 || snapshot.RejectedCount != 0 {
		t.Fatalf("unexpected ignored/rejected counts: %+v", snapshot)
	}
}

func TestBridgeLogsUnmatchedNoticeAsDebugIgnored(t *testing.T) {
	t.Parallel()

	logger, stream := newBridgeTestLogger()
	fakeDispatcher := &recordingDispatcher{deliverable: true}
	eventBridge := New(logger, fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedGroupRecallNoticeEvent())
	if outcome != OutcomeIgnored {
		t.Fatalf("unexpected outcome: got %q want %q", outcome, OutcomeIgnored)
	}

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("expected one bridge log summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.Level != "debug" {
		t.Fatalf("unexpected log level: got %q want debug", summary.Level)
	}
	if summary.Message != "runtime bridge ignored group recall notice" {
		t.Fatalf("unexpected log message: got %q", summary.Message)
	}
	if summary.Details["reason"] != "no plugin subscription accepted the event" {
		t.Fatalf("unexpected ignore reason: %#v", summary.Details)
	}
	if _, ok := summary.Details["error_code"]; ok {
		t.Fatalf("ignored event should not carry error_code: %#v", summary.Details)
	}
}

func TestBridgeIgnoredEventClearsPreviousErrorState(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{
			{PluginID: "weather", Outcome: dispatch.OutcomeDropped},
			{PluginID: "echo", Outcome: dispatch.OutcomeError, ErrorCode: "platform.invalid_request"},
		},
	}
	eventBridge := testBridge(fakeDispatcher)

	if outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent()); outcome != OutcomeError {
		t.Fatalf("unexpected first outcome: got %q want %q", outcome, OutcomeError)
	}

	fakeDispatcher.results = nil
	if outcome := eventBridge.HandleAdapterEvent(context.Background(), supportedGroupRecallNoticeEvent()); outcome != OutcomeIgnored {
		t.Fatalf("unexpected second outcome: got %q want %q", outcome, OutcomeIgnored)
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.LastOutcome != OutcomeIgnored {
		t.Fatalf("unexpected last outcome: got %q want %q", snapshot.LastOutcome, OutcomeIgnored)
	}
	if snapshot.LastErrorCode != "" || snapshot.LastErrorText != "" {
		t.Fatalf("ignored event should clear stale error state: %+v", snapshot)
	}
}

func TestBridgeLogsCommandPolicyRejected(t *testing.T) {
	t.Parallel()

	logger, stream := newBridgeTestLogger()
	eventBridge := New(logger, &recordingDispatcher{deliverable: true})
	observability, unsubscribe := eventBridge.SubscribeObservability(1)
	defer unsubscribe()

	event := supportedAdapterEvent()
	event.PlainText = "/help"
	event.PayloadFields["command"] = "help"

	eventBridge.LogCommandPolicyRejected(event, CommandPolicyRejection{
		CommandName:      "help",
		PluginID:         "raylea.echo",
		MatchedPluginIDs: []string{"raylea.echo"},
		ErrorCode:        "permission.not_whitelisted",
		Reason:           "actor is not whitelisted",
		ReasonSummary:    "sender is not whitelisted",
		PolicyStage:      "whitelist",
	})

	select {
	case frame := <-observability:
		t.Fatalf("unexpected observability frame for rejected command: %#v", frame)
	case <-time.After(100 * time.Millisecond):
	}

	snapshot := eventBridge.Snapshot()
	if snapshot.AcceptedCount != 1 || snapshot.RejectedCount != 1 {
		t.Fatalf("unexpected bridge rejection counters: %+v", snapshot)
	}
	if snapshot.LastOutcome != OutcomeRejected {
		t.Fatalf("unexpected last outcome: got %q want %q", snapshot.LastOutcome, OutcomeRejected)
	}
	if snapshot.LastErrorCode != "permission.not_whitelisted" || snapshot.LastErrorText != "actor is not whitelisted" {
		t.Fatalf("unexpected last rejection details: %+v", snapshot)
	}

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("expected one bridge log summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.Level != "warn" {
		t.Fatalf("unexpected log level: got %q want warn", summary.Level)
	}
	if summary.Source != "bridge" || summary.Protocol != logging.ProtocolOneBot11 {
		t.Fatalf("unexpected log source/protocol: %+v", summary)
	}
	if summary.PluginID != "raylea.echo" {
		t.Fatalf("unexpected plugin_id: got %q want raylea.echo", summary.PluginID)
	}
	if summary.Message != "plugin raylea.echo command help rejected by command policy: sender is not whitelisted" {
		t.Fatalf("unexpected rejection message: got %q", summary.Message)
	}
	if summary.Details["command_name"] != "help" || summary.Details["policy_stage"] != "whitelist" {
		t.Fatalf("unexpected rejection details: %#v", summary.Details)
	}
	if summary.Details["error_code"] != "permission.not_whitelisted" || summary.Details["reason"] != "actor is not whitelisted" {
		t.Fatalf("unexpected rejection details: %#v", summary.Details)
	}
	if !reflect.DeepEqual(summary.Details["matched_plugin_ids"], []any{"raylea.echo"}) {
		t.Fatalf("unexpected matched_plugin_ids detail: %#v", summary.Details["matched_plugin_ids"])
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

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindRequest,
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

func TestBridgeDeliversMetaHeartbeatEvent(t *testing.T) {
	t.Parallel()

	fakeDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "heartbeat-monitor",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	}
	eventBridge := testBridge(fakeDispatcher)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMeta,
		EventID:          "onebot11-meta-heartbeat-1710000456",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "meta.heartbeat",
		Timestamp:        time.Unix(1_700_000_456, 0).Unix(),
		ConversationType: "system",
		ConversationID:   "bot:10001",
		SenderID:         "10001",
		TargetType:       "bot",
		TargetID:         "10001",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":       "meta_event",
				"meta_event_type": "heartbeat",
				"self_id":         "10001",
				"time":            int64(1_700_000_456),
				"interval":        5000,
				"status": map[string]any{
					"online": true,
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
	delivered := fakeDispatcher.events[0]
	if delivered.EventType != "meta.heartbeat" {
		t.Fatalf("unexpected delivered event type: %q", delivered.EventType)
	}
	if delivered.Target == nil || delivered.Target.Type != "bot" || delivered.Target.ID != "10001" {
		t.Fatalf("unexpected target projection: %#v", delivered.Target)
	}
	if delivered.Message != nil {
		t.Fatalf("meta event should not carry a message payload: %#v", delivered.Message)
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

	outcome := eventBridge.HandleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessageSent,
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
					"nickname": "测试用户A",
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

	summary := bridgeEventSummary("queued for dispatcher", adapterintake.NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		TargetName:       "测试群组",
		PlainText:        "测试消息内容",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"sender": map[string]any{
					"nickname": "测试用户昵称",
					"card":     "测试群名片",
					"title":    "管理员",
				},
			},
		},
	})

	if summary != "10001: [测试群组(20001)][管理员]测试群名片/测试用户昵称(30001): 测试消息内容" {
		t.Fatalf("unexpected group summary: %#v", summary)
	}
}

func TestBridgeEventSummaryFormatsPrivateMessageContext(t *testing.T) {
	t.Parallel()

	summary := bridgeEventSummary("queued for dispatcher", adapterintake.NormalizedEvent{
		BotID:          "10001",
		SourceProtocol: "onebot11",
		EventType:      "message.private",
		SenderID:       "30002",
		PlainText:      "你好",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"sender": map[string]any{
					"nickname": "测试私聊用户",
				},
			},
		},
	})

	if summary != "10001: 测试私聊用户(30002): 你好" {
		t.Fatalf("unexpected private summary: %#v", summary)
	}
}

func TestBridgeEventSummaryFormatsFallbackVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		event adapterintake.NormalizedEvent
		want  string
	}{
		{
			name: "group missing group name title and card",
			event: adapterintake.NormalizedEvent{
				BotID:          "10001",
				SourceProtocol: "onebot11",
				EventType:      "message.group",
				ConversationID: "20001",
				SenderID:       "30001",
				PlainText:      "删了",
				PayloadFields: map[string]any{
					"onebot": map[string]any{
						"sender": map[string]any{
							"nickname": "测试用户",
						},
					},
				},
			},
			want: "10001: [20001]测试用户(30001): 删了",
		},
		{
			name: "private missing nickname",
			event: adapterintake.NormalizedEvent{
				BotID:          "10001",
				SourceProtocol: "onebot11",
				EventType:      "message.private",
				SenderID:       "30002",
				PlainText:      "你好",
			},
			want: "10001: 30002(30002): 你好",
		},
		{
			name: "message text truncated",
			event: adapterintake.NormalizedEvent{
				BotID:          "10001",
				SourceProtocol: "onebot11",
				EventType:      "message.private",
				SenderID:       "30002",
				PlainText:      strings.Repeat("终末地", 100),
			},
			want: "10001: 30002(30002): " + strings.Repeat("终末地", 53) + "终...",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := logging.OneBotInboundMessageSummary(logging.OneBotInboundMessageSummaryInput{
				SourceProtocol: tc.event.SourceProtocol,
				BotID:          tc.event.BotID,
				EventType:      tc.event.EventType,
				ConversationID: tc.event.ConversationID,
				SenderID:       tc.event.SenderID,
				PlainText:      tc.event.PlainText,
				PayloadFields:  tc.event.PayloadFields,
			})
			if !ok {
				t.Fatalf("summary should be formatted: %#v", tc.event)
			}
			if got != tc.want {
				t.Fatalf("unexpected summary: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBridgeEventLogAttrsIncludeBotIDAndGroupName(t *testing.T) {
	t.Parallel()

	attrs := bridgeEventLogAttrs(adapterintake.NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		TargetName:       "测试群组",
		PlainText:        "hello bridge",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"self_id": "10001",
				"sender": map[string]any{
					"nickname": "测试用户A",
				},
			},
		},
	})

	attrMap := make(map[string]any, len(attrs)/2)
	for index := 0; index+1 < len(attrs); index += 2 {
		key, _ := attrs[index].(string)
		attrMap[key] = attrs[index+1]
	}

	if attrMap["self_id"] != "10001" {
		t.Fatalf("unexpected self_id attr: %#v", attrMap["self_id"])
	}
	if attrMap["group_name"] != "测试群组" {
		t.Fatalf("unexpected group_name attr: %#v", attrMap["group_name"])
	}
}

type recordingDispatcher struct {
	deliverable bool
	results     []dispatch.DeliveryResult
	events      []runtimeprotocol.Event
	commands    []string
}

func (r *recordingDispatcher) HasDeliverablePlugins() bool {
	return r.deliverable
}

func (r *recordingDispatcher) Dispatch(_ context.Context, event runtimeprotocol.Event, commandName string) []dispatch.DeliveryResult {
	r.events = append(r.events, event)
	r.commands = append(r.commands, commandName)
	return append([]dispatch.DeliveryResult(nil), r.results...)
}

func testBridge(dispatcher dispatcherClient) *Bridge {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, dispatcher)
}

func newBridgeTestLogger() (*slog.Logger, *logging.Stream) {
	stream := logging.NewStream(16)
	writer := logging.NewSummaryWriter(io.Discard, stream, nil)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "msg"
			}
			return attr
		},
	}))
	return logger, stream
}

func supportedAdapterEvent() adapterintake.NormalizedEvent {
	return adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessageText,
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
					"nickname": "测试用户A",
					"card":     "管理员",
					"role":     "admin",
				},
			},
		},
	}
}

func supportedGroupRecallNoticeEvent() adapterintake.NormalizedEvent {
	return adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindNotice,
		EventID:          "onebot11-notice-group-recall-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "notice.group_recall",
		Timestamp:        time.Unix(1_700_000_223, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		MessageID:        "1001",
		PayloadFields: map[string]any{
			"operator_id": "3002",
			"onebot": map[string]any{
				"post_type":    "notice",
				"notice_type":  "group_recall",
				"group_id":     "2001",
				"user_id":      "3001",
				"operator_id":  "3002",
				"message_id":   "1001",
				"time":         int64(1_700_000_223),
				"message_type": "group",
			},
		},
	}
}
