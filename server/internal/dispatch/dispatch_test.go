package dispatch

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

type fakeDeliverer struct {
	mu       sync.Mutex
	events   []runtime.Event
	delivery runtime.Delivery
	err      error
	started  chan runtime.Event
	blockCh  chan struct{} // if non-nil, block until closed
	state    runtime.State
}

func (f *fakeDeliverer) Snapshot() runtime.Snapshot {
	state := f.state
	if state == "" {
		state = runtime.StateRunning
	}
	return runtime.Snapshot{State: state}
}

func (f *fakeDeliverer) DeliverEvent(_ context.Context, event runtime.Event) (runtime.Delivery, error) {
	f.mu.Lock()
	f.events = append(f.events, event)
	f.mu.Unlock()

	if f.started != nil {
		f.started <- event
	}
	if f.blockCh != nil {
		<-f.blockCh
	}
	return f.delivery, f.err
}

func (f *fakeDeliverer) eventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeDeliverer) setState(state runtime.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = state
}

type fakeSender struct {
	mu          sync.Mutex
	messages    []adapter.OutboundMessageSend
	replies     []adapter.OutboundMessageReply
	sendResult  adapter.SendMessageResult
	replyResult adapter.SendMessageResult
	sendErr     error
	replyErr    error
}

func (f *fakeSender) SendMessage(_ context.Context, msg adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
	result := f.sendResult
	if result.MessageID == "" {
		result.MessageID = "msg-1"
	}
	return result, f.sendErr
}

func (f *fakeSender) SendReply(_ context.Context, reply adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	f.mu.Lock()
	f.replies = append(f.replies, reply)
	f.mu.Unlock()
	result := f.replyResult
	if result.MessageID == "" {
		result.MessageID = "reply-1"
	}
	return result, f.replyErr
}

type fakeReplyTargets map[string]outbound.ReplyTarget

func (f fakeReplyTargets) ResolveReplyTarget(eventID string) (outbound.ReplyTarget, bool) {
	target, ok := f[eventID]
	return target, ok
}

type recordingOutboundLimiter struct {
	mu       sync.Mutex
	requests []outbound.MessageLimitRequest
	err      error
}

func (l *recordingOutboundLimiter) Wait(_ context.Context, request outbound.MessageLimitRequest) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, request)
	return l.err
}

func (l *recordingOutboundLimiter) lastRequest() outbound.MessageLimitRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.requests) == 0 {
		return outbound.MessageLimitRequest{}
	}
	return l.requests[len(l.requests)-1]
}

func testEvent() runtime.Event {
	return runtime.Event{
		EventID:        "test-evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor:          &runtime.EventActor{ID: "100", Nickname: "Alice"},
		Target:         &runtime.EventTarget{Type: "group", ID: "200", Name: "测试群"},
		Message:        &runtime.EventMessage{PlainText: "hello"},
	}
}

func testEventWithCommand(commandName string) runtime.Event {
	event := testEvent()
	event.PayloadFields = map[string]any{
		"command": commandName,
	}
	return event
}

func testEventWithTarget(targetID string) runtime.Event {
	event := testEvent()
	event.EventID = "test-evt-" + targetID
	event.Target = &runtime.EventTarget{Type: "group", ID: targetID}
	return event
}

func waitForStartedEvent(t *testing.T, started <-chan runtime.Event) runtime.Event {
	t.Helper()

	select {
	case event := <-started:
		return event
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event delivery to start")
		return runtime.Event{}
	}
}

func waitForCondition(t *testing.T, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func newDispatchTestLogger() (*slog.Logger, *logging.Stream) {
	stream := logging.NewStream(16)
	writer := logging.NewSummaryWriter(io.Discard, stream, nil)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
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

func waitForDispatchLog(t *testing.T, stream *logging.Stream, match func(logging.Summary) bool) logging.Summary {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, summary := range stream.Snapshot() {
			if match(summary) {
				return summary
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	var buffer bytes.Buffer
	for _, summary := range stream.Snapshot() {
		buffer.WriteString(summary.Message)
		buffer.WriteByte('\n')
	}
	t.Fatalf("timed out waiting for dispatch log; captured messages:\n%s", buffer.String())
	return logging.Summary{}
}

func TestDispatchFanOutToMultiplePlugins(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("plugin-a", rt1, nil, nil, 1)
	d.Register("plugin-b", rt2, nil, nil, 1)

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Wait for workers to process.
	time.Sleep(100 * time.Millisecond)

	if rt1.eventCount() != 1 || rt2.eventCount() != 1 {
		t.Errorf("expected 1 event each, got plugin-a=%d, plugin-b=%d", rt1.eventCount(), rt2.eventCount())
	}
}

func TestDispatchDirectedDeliveryByCommand(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("weather", rt1, nil, []CommandDecl{
		{Name: "weather", Aliases: []string{"天气"}},
	}, 1)
	d.Register("echo", rt2, nil, []CommandDecl{
		{Name: "echo"},
	}, 1)

	results := d.Dispatch(context.Background(), testEvent(), "weather")
	if len(results) != 1 {
		t.Fatalf("expected 1 directed result, got %d", len(results))
	}
	if results[0].PluginID != "weather" {
		t.Errorf("expected plugin weather, got %s", results[0].PluginID)
	}

	time.Sleep(50 * time.Millisecond)
	if rt1.eventCount() != 1 {
		t.Errorf("weather plugin should receive 1 event, got %d", rt1.eventCount())
	}
	if rt2.eventCount() != 0 {
		t.Errorf("echo plugin should receive 0 events, got %d", rt2.eventCount())
	}
}

func TestDispatchDirectedDeliveryByAlias(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	d.Register("weather", rt1, nil, []CommandDecl{
		{Name: "weather", Aliases: []string{"天气"}},
	}, 1)

	results := d.Dispatch(context.Background(), testEvent(), "天气")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestDispatchFallbackWhenNoCommandMatch(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("plugin-a", rt1, nil, nil, 1)
	d.Register("plugin-b", rt2, nil, nil, 1)

	results := d.Dispatch(context.Background(), testEvent(), "unknown_command")
	if len(results) != 2 {
		t.Fatalf("expected 2 fallback results, got %d", len(results))
	}
}

func TestDispatchSubscriptionFiltering(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("msg-only", rt1, []string{"message.group", "message.private"}, nil, 1)
	d.Register("notice-only", rt2, []string{"notice.member_increase"}, nil, 1)

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (msg-only), got %d", len(results))
	}
	if results[0].PluginID != "msg-only" {
		t.Errorf("expected msg-only, got %s", results[0].PluginID)
	}
}

func TestDispatchSkipsNonRunningRuntimes(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rtRunning := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rtBackoff := &fakeDeliverer{
		state:    runtime.StateBackoff,
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
	}

	d.Register("running", rtRunning, nil, nil, 1)
	d.Register("backoff", rtBackoff, nil, nil, 1)

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 1 {
		t.Fatalf("expected only one deliverable target, got %d", len(results))
	}
	if results[0].PluginID != "running" {
		t.Fatalf("unexpected target: got %q want %q", results[0].PluginID, "running")
	}

	time.Sleep(50 * time.Millisecond)
	if rtRunning.eventCount() != 1 {
		t.Fatalf("running runtime should receive the event, got %d", rtRunning.eventCount())
	}
	if rtBackoff.eventCount() != 0 {
		t.Fatalf("backoff runtime should not receive the event, got %d", rtBackoff.eventCount())
	}
	if !d.HasDeliverablePlugin("running") {
		t.Fatal("running runtime should be deliverable")
	}
	if d.HasDeliverablePlugin("backoff") {
		t.Fatal("backoff runtime should not be deliverable")
	}
	if !d.HasDeliverablePlugins() {
		t.Fatal("dispatcher should report at least one deliverable runtime")
	}
}

func TestDispatchQueueOverflow(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 1)
	defer d.Close()

	blocker := &fakeDeliverer{
		blockCh:  make(chan struct{}),
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
	}
	d.Register("blocker", blocker, nil, nil, 1)

	// First dispatch fills the single-capacity queue.
	d.Dispatch(context.Background(), testEvent(), "")
	// Give the worker time to pick up the first item and block.
	time.Sleep(20 * time.Millisecond)
	// Now the queue is empty but the worker is blocked. Fill queue again.
	d.Dispatch(context.Background(), testEvent(), "")
	// Third should be dropped.
	results := d.Dispatch(context.Background(), testEvent(), "")

	hasDropped := false
	for _, r := range results {
		if r.Outcome == OutcomeDropped {
			hasDropped = true
		}
	}
	if !hasDropped {
		t.Error("expected at least one dropped outcome")
	}

	close(blocker.blockCh)
}

func TestDispatchDifferentTargetsRunConcurrently(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtime.Event, 2),
		blockCh:  make(chan struct{}),
	}
	d.Register("parallel", rt, nil, nil, 2)

	d.Dispatch(context.Background(), testEventWithTarget("200"), "")
	d.Dispatch(context.Background(), testEventWithTarget("201"), "")

	first := waitForStartedEvent(t, rt.started)
	second := waitForStartedEvent(t, rt.started)
	if first.Target == nil || second.Target == nil {
		t.Fatalf("unexpected started events: %#v %#v", first, second)
	}
	if first.Target.ID == second.Target.ID {
		t.Fatalf("expected different lanes, got %#v and %#v", first.Target, second.Target)
	}

	close(rt.blockCh)
}

func TestDispatchSameTargetPreservesFIFO(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtime.Event, 2),
		blockCh:  make(chan struct{}),
	}
	d.Register("ordered", rt, nil, nil, 2)

	firstEvent := testEventWithTarget("200")
	secondEvent := testEventWithTarget("200")
	secondEvent.EventID = "test-evt-200-second"

	d.Dispatch(context.Background(), firstEvent, "")
	startedFirst := waitForStartedEvent(t, rt.started)
	if startedFirst.EventID != firstEvent.EventID {
		t.Fatalf("unexpected first started event: %#v", startedFirst)
	}

	d.Dispatch(context.Background(), secondEvent, "")
	select {
	case startedSecond := <-rt.started:
		t.Fatalf("second event started before first lane drained: %#v", startedSecond)
	case <-time.After(80 * time.Millisecond):
	}

	close(rt.blockCh)

	startedSecond := waitForStartedEvent(t, rt.started)
	if startedSecond.EventID != secondEvent.EventID {
		t.Fatalf("unexpected second started event: %#v", startedSecond)
	}
}

func TestDispatchDeregister(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	d.Register("test", rt, nil, nil, 1)
	d.Deregister("test")

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 0 {
		t.Fatalf("expected 0 results after deregister, got %d", len(results))
	}
}

func TestDispatchDeregisterWaitsForActiveLane(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtime.Event, 1),
		blockCh:  make(chan struct{}),
	}
	d.Register("test", rt, nil, nil, 2)

	d.Dispatch(context.Background(), testEventWithTarget("200"), "")
	waitForStartedEvent(t, rt.started)

	done := make(chan struct{})
	go func() {
		d.Deregister("test")
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("deregister returned before the active lane drained")
	case <-time.After(80 * time.Millisecond):
	}

	close(rt.blockCh)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("deregister did not finish after the active lane drained")
	}
}

func TestDispatchToPluginRejectsNonRunningRuntime(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{
		state:    runtime.StateBackoff,
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
	}
	d.Register("test", rt, nil, nil, 1)

	result := d.DispatchToPlugin(context.Background(), "test", testEvent())
	if result.Outcome != OutcomeError {
		t.Fatalf("unexpected outcome: got %q want %q", result.Outcome, OutcomeError)
	}
	if result.ErrorCode != "platform.invalid_request" {
		t.Fatalf("unexpected error code: got %q want %q", result.ErrorCode, "platform.invalid_request")
	}

	time.Sleep(20 * time.Millisecond)
	if rt.eventCount() != 0 {
		t.Fatalf("non-running runtime should not receive the event, got %d", rt.eventCount())
	}
}

func TestDispatchSkipsQueuedEventWhenRuntimeStopsBeforeDelivery(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
		blockCh:  make(chan struct{}),
	}
	d.Register("test", rt, nil, nil, 1)

	results := d.Dispatch(context.Background(), testEventWithTarget("200"), "")
	if len(results) != 1 || results[0].Outcome != OutcomeDelivered {
		t.Fatalf("unexpected first dispatch result: %#v", results)
	}
	waitForCondition(t, func() bool { return rt.eventCount() == 1 }, "first event should start delivery")

	results = d.Dispatch(context.Background(), testEventWithTarget("201"), "")
	if len(results) != 1 || results[0].Outcome != OutcomeDelivered {
		t.Fatalf("unexpected queued dispatch result: %#v", results)
	}
	rt.setState(runtime.StateStarting)
	close(rt.blockCh)

	time.Sleep(80 * time.Millisecond)
	if got := rt.eventCount(); got != 1 {
		t.Fatalf("stopped runtime should not receive queued event, got %d events", got)
	}
}

func TestDispatchActionExecution(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "reply text"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(100 * time.Millisecond)

	sender.mu.Lock()
	count := len(sender.messages)
	sender.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 sent message, got %d", count)
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages[0].Segments) != 1 || sender.messages[0].Segments[0].Type != "text" {
		t.Fatalf("unexpected sent message payload: %#v", sender.messages[0])
	}
}

func TestDispatchActionExecutionWithRichSegments(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{
				{Type: "at", Data: map[string]any{"user_id": "300"}},
				{Type: "text", Data: map[string]any{"text": " rich dispatch"}},
			},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(100 * time.Millisecond)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.messages))
	}
	if len(sender.messages[0].Segments) != 2 {
		t.Fatalf("unexpected rich segments: %#v", sender.messages[0])
	}
}

func TestDispatchActionExecutionUsesReplyTargetForOutboundLimiter(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	limiter := &recordingOutboundLimiter{}
	d := New(slog.Default(), sender, fakeReplyTargets{
		"evt_reply_target": {
			MessageID:  "msg-1",
			TargetType: "group",
			TargetID:   "200",
		},
	}, 16)
	d.SetOutboundLimiter(limiter)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		Action: &runtime.Action{
			Kind:           "message.reply",
			ReplyToEventID: "evt_reply_target",
			MessageSegments: []runtime.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "reply text"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(100 * time.Millisecond)

	request := limiter.lastRequest()
	if request.PluginID != "action-plugin" || request.TargetType != "group" || request.TargetID != "200" {
		t.Fatalf("unexpected limiter request: %#v", request)
	}
}

func TestDispatchActionExecutionLogsRateLimitedOutcome(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{}
	limiter := &recordingOutboundLimiter{
		err: &adapter.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"},
	}
	d := New(logger, sender, nil, 16)
	d.SetOutboundLimiter(limiter)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_rate_limited",
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
				Type: "text",
				Data: map[string]any{"text": "limited"},
			}},
		},
	}}
	d.Register("action-plugin", rt, nil, nil, 1)

	d.Dispatch(context.Background(), testEventWithCommand("echo"), "")

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return summary.RequestID == "req_runtime_delivery_rate_limited"
	})
	if summary.Details["error_code"] != "platform.rate_limited" {
		t.Fatalf("unexpected error code: %#v", summary.Details["error_code"])
	}

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 0 || len(sender.replies) != 0 {
		t.Fatalf("rate limited action should not send: messages=%#v replies=%#v", sender.messages, sender.replies)
	}
}

func TestDispatchActionExecutionRejectsMissingMessageSendCapability(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	sender := &fakeSender{}
	d := New(logger, sender, nil, 16)
	d.SetCapabilityChecker(func(_ context.Context, pluginID, capability string) bool {
		return false
	})
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_permission_send",
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
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
	if summary.Details["error_code"] != "permission.scope_violation" {
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

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_permission_reply",
		Action: &runtime.Action{
			Kind:           "message.reply",
			ReplyToEventID: "evt_reply_target",
			MessageSegments: []runtime.ActionSegment{{
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
	if summary.Details["error_code"] != "permission.scope_violation" {
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
		sendResult: adapter.SendMessageResult{MessageID: "send-100"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_0001",
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
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
		sendErr: &adapter.Error{Code: "adapter.send_failed", Message: "send rejected by upstream"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_0002",
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
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
		replyErr:   &adapter.Error{Code: "adapter.reply_target_missing", Message: "reply target missing"},
		sendResult: adapter.SendMessageResult{MessageID: "send-200"},
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

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_0003",
		Action: &runtime.Action{
			Kind:                    "message.reply",
			ReplyToEventID:          "evt_reply_target",
			FallbackToSendIfMissing: true,
			MessageSegments: []runtime.ActionSegment{{
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
		sendResult: adapter.SendMessageResult{MessageID: "send-300"},
	}
	d := New(logger, sender, nil, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		RequestID: "req_runtime_delivery_0004",
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtime.ActionSegment{{
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

	rt := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{}}}
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

	noTarget := runtime.Event{
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
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
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
