package dispatch

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
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

type fakeSender struct {
	mu       sync.Mutex
	messages []adapter.OutboundMessageSend
}

func (f *fakeSender) SendMessage(_ context.Context, msg adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
	return adapter.SendMessageResult{MessageID: "msg-1"}, nil
}

func (f *fakeSender) SendReply(_ context.Context, _ adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	return adapter.SendMessageResult{}, nil
}

func testEvent() runtime.Event {
	return runtime.Event{
		EventID:        "test-evt-1",
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor:          &runtime.EventActor{ID: "100"},
		Target:         &runtime.EventTarget{Type: "group", ID: "200"},
		Message:        &runtime.EventMessage{PlainText: "hello"},
	}
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
