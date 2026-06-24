package dispatch

import (
	"context"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestDispatchActionExecutionRejectsMissingMessageSendCapability(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{}
	d := New(logger, sender, nil, 16)
	d.SetCapabilityChecker(func(_ context.Context, pluginID, capability string) bool {
		return false
	})
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_permission_send",
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "should be denied"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_permission_send"
	})
	if summary.Details["error_code"] != "plugin.capability_violation" {
		t.Fatalf("unexpected error code: %#v", summary.Details["error_code"])
	}

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 0 {
		t.Fatalf("unexpected outbound sends: %#v", sender.messages)
	}
	if len(sender.replies) != 0 {
		t.Fatalf("unexpected outbound replies: %#v", sender.replies)
	}
}

func TestDispatchActionExecutionRejectsMissingMessageReplyCapability(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{}
	d := New(logger, sender, fakeReplyTargets{
		"evt_reply_target": {
			MessageID:  "msg-1",
			TargetType: "group",
			TargetID:   "200",
		},
	}, 16)
	d.SetCapabilityChecker(func(_ context.Context, pluginID, capability string) bool {
		return false
	})
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_permission_reply",
		Action: &runtimeaction.Action{
			Kind:           "message.reply",
			ReplyToEventID: "evt_reply_target",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "reply denied"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_permission_reply"
	})
	if summary.Details["error_code"] != "plugin.capability_violation" {
		t.Fatalf("unexpected error code: %#v", summary.Details["error_code"])
	}

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 0 {
		t.Fatalf("unexpected outbound sends: %#v", sender.messages)
	}
	if len(sender.replies) != 0 {
		t.Fatalf("unexpected outbound replies: %#v", sender.replies)
	}
}

func TestDispatchLogsOutboundMessageSuccess(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{
		sendResult: adapteroutbound.SendMessageResult{MessageID: "send-100"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_0001",
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "hello dispatch"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_0001"
	})
	if summary.Level != "info" {
		t.Fatalf("unexpected log level: got %q want info", summary.Level)
	}
	if summary.Source != "adapter.onebot11" {
		t.Fatalf("unexpected log source: got %q want adapter.onebot11", summary.Source)
	}
	if summary.Protocol != logging.ProtocolOneBot11 {
		t.Fatalf("unexpected protocol: got %q want %q", summary.Protocol, logging.ProtocolOneBot11)
	}
	if summary.Message != "action-plugin/echo -> [测试群(200)]：hello dispatch" {
		t.Fatalf("unexpected log message: got %q", summary.Message)
	}
	if summary.PluginID != "action-plugin" {
		t.Fatalf("unexpected plugin_id: got %q want action-plugin", summary.PluginID)
	}
	if summary.Details["direction"] != "outbound" {
		t.Fatalf("unexpected direction detail: %#v", summary.Details)
	}
	if summary.Details["action_kind"] != "message.send" || summary.Details["delivery_kind"] != "message.send" {
		t.Fatalf("unexpected delivery details: %#v", summary.Details)
	}
	if summary.Details["command_name"] != "echo" {
		t.Fatalf("unexpected command_name detail: %#v", summary.Details["command_name"])
	}
	if summary.Details["target_type"] != "group" || summary.Details["target_id"] != "200" {
		t.Fatalf("unexpected target details: %#v", summary.Details)
	}
	if summary.Details["plain_text"] != "hello dispatch" {
		t.Fatalf("unexpected plain_text detail: %#v", summary.Details["plain_text"])
	}
	if summary.Details["message_id"] != "send-100" {
		t.Fatalf("unexpected message_id detail: %#v", summary.Details["message_id"])
	}
}

func TestDispatchLogsOutboundMessageFailure(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{
		sendErr: &adapteroutbound.Error{Code: "adapter.send_failed", Message: "send rejected by upstream"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_0002",
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "hello dispatch"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_0002"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected log level: got %q want warn", summary.Level)
	}
	if summary.Message != "action-plugin/echo -> [测试群(200)] 发送失败：hello dispatch" {
		t.Fatalf("unexpected log message: got %q", summary.Message)
	}
	if summary.Details["command_name"] != "echo" {
		t.Fatalf("unexpected command_name detail: %#v", summary.Details["command_name"])
	}
	if summary.Details["error_code"] != "adapter.send_failed" {
		t.Fatalf("unexpected error_code detail: %#v", summary.Details["error_code"])
	}
	if summary.Details["reason"] != "send rejected by upstream" {
		t.Fatalf("unexpected reason detail: %#v", summary.Details["reason"])
	}
}

func TestDispatchLogsReplyFallbackUsingActualDeliveryKind(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{
		replyErr:   &adapteroutbound.Error{Code: "adapter.reply_target_missing", Message: "reply target missing"},
		sendResult: adapteroutbound.SendMessageResult{MessageID: "send-200"},
	}
	resolver := fakeReplyTargets{
		"evt_reply_target": {
			MessageID:  "msg-1",
			TargetType: "group",
			TargetID:   "200",
		},
	}
	d := New(logger, sender, resolver, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_0003",
		Action: &runtimeaction.Action{
			Kind:                    "message.reply",
			ReplyToEventID:          "evt_reply_target",
			FallbackToSendIfMissing: true,
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "fallback reply"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_0003"
	})
	if summary.Level != "info" {
		t.Fatalf("unexpected log level: got %q want info", summary.Level)
	}
	if summary.Details["action_kind"] != "message.reply" {
		t.Fatalf("unexpected action_kind detail: %#v", summary.Details["action_kind"])
	}
	if summary.Details["delivery_kind"] != "message.send" {
		t.Fatalf("unexpected delivery_kind detail: %#v", summary.Details["delivery_kind"])
	}
	if summary.Details["command_name"] != "echo" {
		t.Fatalf("unexpected command_name detail: %#v", summary.Details["command_name"])
	}
	if summary.Message != "action-plugin/echo -> [测试群(200)]：fallback reply" {
		t.Fatalf("unexpected fallback summary: got %q", summary.Message)
	}
	if summary.Details["target_type"] != "group" || summary.Details["target_id"] != "200" {
		t.Fatalf("unexpected fallback target details: %#v", summary.Details)
	}
	if summary.Details["message_id"] != "send-200" {
		t.Fatalf("unexpected fallback message_id detail: %#v", summary.Details["message_id"])
	}
}

func TestDispatchLogsOutboundMessageWithoutCommandContext(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{
		sendResult: adapteroutbound.SendMessageResult{MessageID: "send-300"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_0004",
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "hello dispatch"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEvent(), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_0004"
	})
	if summary.Message != "action-plugin -> [测试群(200)]：hello dispatch" {
		t.Fatalf("unexpected log message: got %q", summary.Message)
	}
	if _, ok := summary.Details["command_name"]; ok {
		t.Fatalf("unexpected command_name detail: %#v", summary.Details["command_name"])
	}
}

func TestDispatcherWindowFlushPublishesDeltas(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{}}}
	d.Register("p", rt, []string{"message.group"}, nil, 1)

	pub := &recordingRuntimePublisher{}
	d.SetRuntimePublisher(pub)

	d.Dispatch(context.Background(), testEvent(), "")
	d.FlushDispatcherWindow(5)

	snapshots := pub.Snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	first := snapshots[0]
	if first.WindowSeconds != 5 || first.Delivered != 1 || first.Dropped != 0 || first.Ignored != 0 {
		t.Fatalf("unexpected snapshot: %+v", first)
	}

	noTarget := runtimeprotocol.Event{
		EventID:        "evt-no-target",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "notice.member_increase",
	}
	d.Dispatch(context.Background(), noTarget, "")
	d.FlushDispatcherWindow(5)

	snapshots = pub.Snapshots()
	if len(snapshots) != 2 {
		t.Fatalf("expected two snapshots, got %d", len(snapshots))
	}
	second := snapshots[1]
	if second.Delivered != 0 || second.Ignored != 1 {
		t.Fatalf("expected delta Ignored=1 Delivered=0, got %+v", second)
	}
}

func TestDispatcherFlushDropsByReasonRecordsQueueFull(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 1)
	defer d.Close()

	blocker := &fakeDeliverer{
		blockCh:  make(chan struct{}),
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
	}
	d.Register("blocker", blocker, nil, nil, 1)

	pub := &recordingRuntimePublisher{}
	d.SetRuntimePublisher(pub)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(20 * time.Millisecond)
	d.Dispatch(context.Background(), testEvent(), "")
	d.Dispatch(context.Background(), testEvent(), "")
	d.FlushDispatcherWindow(10)

	snapshots := pub.Snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	snap := snapshots[0]
	foundQueueFull := false
	for _, row := range snap.DropsByReason {
		if row.Reason == "queue_full" && row.PluginID == "blocker" && row.Count >= 1 {
			foundQueueFull = true
		}
	}
	if !foundQueueFull {
		t.Fatalf("expected queue_full drop row for blocker, got %+v", snap.DropsByReason)
	}

	close(blocker.blockCh)
}

type recordingRuntimePublisher struct {
	mu        sync.Mutex
	snapshots []DispatcherWindowSnapshot
}

func (p *recordingRuntimePublisher) PublishDispatcherRuntime(snap DispatcherWindowSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshots = append(p.snapshots, snap)
}

func (p *recordingRuntimePublisher) Snapshots() []DispatcherWindowSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]DispatcherWindowSnapshot, len(p.snapshots))
	copy(out, p.snapshots)
	return out
}

// recordingDispatchMetrics captures dispatcher metric callbacks so the
// outbound-instrumentation test can assert IncOutboundSend and
// ObserveOutboundDuration are invoked once per send attempt.

type recordingDispatchMetrics struct {
	mu                sync.Mutex
	pipelineCounters  map[string]map[string]int
	dispatcherDrops   map[string]map[string]int
	outboundSends     map[string]map[string]int
	outboundDurations []outboundDurationSample
}

type outboundDurationSample struct {
	adapter  string
	duration time.Duration
}

func newRecordingDispatchMetrics() *recordingDispatchMetrics {
	return &recordingDispatchMetrics{
		pipelineCounters: map[string]map[string]int{},
		dispatcherDrops:  map[string]map[string]int{},
		outboundSends:    map[string]map[string]int{},
	}
}

func (m *recordingDispatchMetrics) IncDispatcherDrop(pluginID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.dispatcherDrops[pluginID]; !ok {
		m.dispatcherDrops[pluginID] = map[string]int{}
	}
	m.dispatcherDrops[pluginID][reason]++
}

func (m *recordingDispatchMetrics) IncEventPipelineStage(stage, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pipelineCounters[stage]; !ok {
		m.pipelineCounters[stage] = map[string]int{}
	}
	m.pipelineCounters[stage][outcome]++
}

func (m *recordingDispatchMetrics) IncOutboundSend(adapter, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.outboundSends[adapter]; !ok {
		m.outboundSends[adapter] = map[string]int{}
	}
	m.outboundSends[adapter][outcome]++
}

func (m *recordingDispatchMetrics) ObserveOutboundDuration(adapter string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outboundDurations = append(m.outboundDurations, outboundDurationSample{adapter: adapter, duration: duration})
}

// TestDispatchActionExecutionRecordsOutboundMetrics verifies that the
// dispatcher records outbound send latency and outcome counters every
// time an action is sent. The /api/system/metrics contract advertises
// outbound_send_total and outbound_send_duration_seconds and depends on
// these calls firing in production.

func TestDispatchActionExecutionRecordsOutboundMetrics(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	metrics := newRecordingDispatchMetrics()
	d.SetMetricsObserver(metrics)

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "metric reply"},
			}},
		},
	}}
	d.Register("metric-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(100 * time.Millisecond)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if got := metrics.outboundSends["onebot11"]["delivered"]; got != 1 {
		t.Fatalf("delivered count = %d, want 1; sends=%#v", got, metrics.outboundSends)
	}
	if len(metrics.outboundDurations) != 1 {
		t.Fatalf("duration samples = %d, want 1", len(metrics.outboundDurations))
	}
	if metrics.outboundDurations[0].adapter != "onebot11" {
		t.Fatalf("adapter = %q, want onebot11", metrics.outboundDurations[0].adapter)
	}
}
