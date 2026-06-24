package dispatch

import (
	"bytes"
	"context"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeDeliverer struct {
	mu       sync.Mutex
	events   []runtimeprotocol.Event
	delivery runtimemanager.Delivery
	err      error
	started  chan runtimeprotocol.Event
	blockCh  chan struct{} // if non-nil, block until closed
	state    runtimemanager.State
}

type recordingSchedulerRunRecorder struct {
	mu      sync.Mutex
	entries []runtimeprotocol.SchedulerRunResult
}

func (r *recordingSchedulerRunRecorder) RecordSchedulerRunResult(_ context.Context, result runtimeprotocol.SchedulerRunResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, result)
	return nil
}

func (r *recordingSchedulerRunRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

func (r *recordingSchedulerRunRecorder) results() []runtimeprotocol.SchedulerRunResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]runtimeprotocol.SchedulerRunResult(nil), r.entries...)
}

func (f *fakeDeliverer) Snapshot() runtimemanager.Snapshot {
	state := f.state
	if state == "" {
		state = runtimemanager.StateRunning
	}
	return runtimemanager.Snapshot{State: state}
}

func (f *fakeDeliverer) DeliverEvent(_ context.Context, event runtimeprotocol.Event) (runtimemanager.Delivery, error) {
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

func (f *fakeDeliverer) setState(state runtimemanager.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = state
}

type fakeSender struct {
	mu          sync.Mutex
	messages    []adapteroutbound.OutboundMessageSend
	replies     []adapteroutbound.OutboundMessageReply
	sendResult  adapteroutbound.SendMessageResult
	replyResult adapteroutbound.SendMessageResult
	sendErr     error
	replyErr    error
}

func (f *fakeSender) SendMessage(_ context.Context, msg adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
	result := f.sendResult
	if result.MessageID == "" {
		result.MessageID = "msg-1"
	}
	return result, f.sendErr
}

func (f *fakeSender) SendReply(_ context.Context, reply adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error) {
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

func testEvent() runtimeprotocol.Event {
	return runtimeprotocol.Event{
		EventID:        "test-evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor:          &runtimeprotocol.EventActor{ID: "100", Nickname: "测试用户A"},
		Target:         &runtimeprotocol.EventTarget{Type: "group", ID: "200", Name: "测试群"},
		Message:        &runtimeprotocol.EventMessage{PlainText: "hello"},
	}
}

func testEventWithCommand(commandName string) runtimeprotocol.Event {
	event := testEvent()
	event.PayloadFields = map[string]any{
		"command": commandName,
	}
	return event
}

func testEventWithTarget(targetID string) runtimeprotocol.Event {
	event := testEvent()
	event.EventID = "test-evt-" + targetID
	event.Target = &runtimeprotocol.EventTarget{Type: "group", ID: targetID}
	return event
}

func waitForStartedEvent(t *testing.T, started <-chan runtimeprotocol.Event) runtimeprotocol.Event {
	t.Helper()

	select {
	case event := <-started:
		return event
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event delivery to start")
		return runtimeprotocol.Event{}
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

func findDispatchLog(stream *logging.Stream, match func(logging.Summary) bool) *logging.Summary {
	for _, summary := range stream.Snapshot() {
		if match(summary) {
			matched := summary
			return &matched
		}
	}
	return nil
}

func TestDispatchFanOutToMultiplePlugins(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}

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

func TestDispatchRecordsSchedulerSuccessWithoutCompletionLog(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	d := New(logger, nil, nil, 1)
	defer d.Close()
	recorder := &recordingSchedulerRunRecorder{}

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"handled": true}}}
	d.Register("weather", rt, []string{"scheduler.trigger"}, nil, 1)

	result := d.DispatchToPlugin(context.Background(), "weather", runtimeprotocol.Event{
		EventID:        "scheduler-daily_report-1",
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      time.Now().Unix(),
		SchedulerLog: &runtimeprotocol.SchedulerLogContext{
			JobID:      "daily_report",
			PluginName: "天气插件",
			TaskName:   "daily_report",
			LogLabel:   "每日早报",
			StartedAt:  time.Now().Add(-150 * time.Millisecond),
			Recorder:   recorder,
		},
	})
	if result.Outcome != OutcomeDelivered {
		t.Fatalf("DispatchToPlugin outcome = %s, want delivered", result.Outcome)
	}

	waitForCondition(t, func() bool {
		return recorder.count() == 1
	}, "scheduler success should be recorded")
	got := recorder.results()[0]
	if got.JobID != "daily_report" || got.Outcome != "success" {
		t.Fatalf("unexpected scheduler run result: %#v", got)
	}
	if summary := findDispatchLog(stream, func(summary logging.Summary) bool {
		return strings.Contains(summary.Message, "处理完成")
	}); summary != nil {
		t.Fatalf("success completion should not be logged: %#v", summary)
	}
}

func TestDispatchLogsAndRecordsSchedulerFailure(t *testing.T) {
	t.Parallel()

	logger, stream := newDispatchTestLogger()
	d := New(logger, nil, nil, 1)
	defer d.Close()
	recorder := &recordingSchedulerRunRecorder{}

	rt := &fakeDeliverer{err: &runtimemanager.Error{Code: "plugin.event_timeout", Message: "plugin event response timed out"}}
	d.Register("weather", rt, []string{"scheduler.trigger"}, nil, 1)

	result := d.DispatchToPlugin(context.Background(), "weather", runtimeprotocol.Event{
		EventID:        "scheduler-daily_report-2",
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      time.Now().Unix(),
		SchedulerLog: &runtimeprotocol.SchedulerLogContext{
			JobID:      "daily_report",
			PluginName: "天气插件",
			TaskName:   "daily_report",
			LogLabel:   "每日早报",
			StartedAt:  time.Now().Add(-150 * time.Millisecond),
			Recorder:   recorder,
		},
	})
	if result.Outcome != OutcomeDelivered {
		t.Fatalf("DispatchToPlugin outcome = %s, want delivered", result.Outcome)
	}

	summary := waitForDispatchLog(t, stream, func(summary logging.Summary) bool {
		return strings.Contains(summary.Message, "【天气插件｜daily_report｜每日早报｜处理失败】耗时 ")
	})
	if summary.Source != "scheduler" || summary.PluginID != "weather" {
		t.Fatalf("unexpected failure log: %#v", summary)
	}
	waitForCondition(t, func() bool {
		return recorder.count() == 1
	}, "scheduler failure should be recorded")
	got := recorder.results()[0]
	if got.JobID != "daily_report" || got.Outcome != "timeout" || got.ErrorCode != "plugin.event_timeout" {
		t.Fatalf("unexpected scheduler run result: %#v", got)
	}
}

func TestDispatchDirectedDeliveryByCommand(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}

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

	rt1 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
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

	rt1 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}

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

	rt1 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}

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

	rtRunning := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
	rtBackoff := &fakeDeliverer{
		state:    runtimemanager.StateBackoff,
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
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
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
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
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtimeprotocol.Event, 2),
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
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtimeprotocol.Event, 2),
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

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}}}
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
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
		started:  make(chan runtimeprotocol.Event, 1),
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
		state:    runtimemanager.StateBackoff,
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
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
		delivery: runtimemanager.Delivery{Result: map[string]any{"ok": true}},
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
	rt.setState(runtimemanager.StateStarting)
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

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
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

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{
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

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		Action: &runtimeaction.Action{
			Kind:           "message.reply",
			ReplyToEventID: "evt_reply_target",
			MessageSegments: []runtimeaction.ActionSegment{{
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
		err: &adapteroutbound.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"},
	}
	d := New(logger, sender, nil, 16)
	d.SetOutboundLimiter(limiter)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtimemanager.Delivery{
		RequestID: "req_runtime_delivery_rate_limited",
		Action: &runtimeaction.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			MessageSegments: []runtimeaction.ActionSegment{{
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
