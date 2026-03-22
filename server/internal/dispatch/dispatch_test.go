package dispatch

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/runtime"
)

type fakeDeliverer struct {
	mu       sync.Mutex
	events   []runtime.Event
	delivery runtime.Delivery
	err      error
	blockCh  chan struct{} // if non-nil, block until closed
}

func (f *fakeDeliverer) Snapshot() runtime.Snapshot {
	return runtime.Snapshot{State: runtime.StateRunning}
}

func (f *fakeDeliverer) DeliverEvent(_ context.Context, event runtime.Event) (runtime.Delivery, error) {
	if f.blockCh != nil {
		<-f.blockCh
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
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

func (f *fakeSender) SendImage(_ context.Context, _ adapter.OutboundMessageSendImage) (adapter.SendMessageResult, error) {
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

func TestDispatchFanOutToMultiplePlugins(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("plugin-a", rt1, nil, nil)
	d.Register("plugin-b", rt2, nil, nil)

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
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("weather", rt1, nil, []CommandDecl{
		{Name: "weather", Aliases: []string{"天气"}},
	})
	d.Register("echo", rt2, nil, []CommandDecl{
		{Name: "echo"},
	})

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
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	d.Register("weather", rt1, nil, []CommandDecl{
		{Name: "weather", Aliases: []string{"天气"}},
	})

	results := d.Dispatch(context.Background(), testEvent(), "天气")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestDispatchFallbackWhenNoCommandMatch(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("plugin-a", rt1, nil, nil)
	d.Register("plugin-b", rt2, nil, nil)

	results := d.Dispatch(context.Background(), testEvent(), "unknown_command")
	if len(results) != 2 {
		t.Fatalf("expected 2 fallback results, got %d", len(results))
	}
}

func TestDispatchSubscriptionFiltering(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt1 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	rt2 := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}

	d.Register("msg-only", rt1, []string{"message.group", "message.private"}, nil)
	d.Register("notice-only", rt2, []string{"notice.member_increase"}, nil)

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (msg-only), got %d", len(results))
	}
	if results[0].PluginID != "msg-only" {
		t.Errorf("expected msg-only, got %s", results[0].PluginID)
	}
}

func TestDispatchQueueOverflow(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 1)
	defer d.Close()

	blocker := &fakeDeliverer{
		blockCh:  make(chan struct{}),
		delivery: runtime.Delivery{Result: map[string]any{"ok": true}},
	}
	d.Register("blocker", blocker, nil, nil)

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

func TestDispatchDeregister(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"ok": true}}}
	d.Register("test", rt, nil, nil)
	d.Deregister("test")

	results := d.Dispatch(context.Background(), testEvent(), "")
	if len(results) != 0 {
		t.Fatalf("expected 0 results after deregister, got %d", len(results))
	}
}

func TestDispatchActionExecution(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, 16)
	defer d.Close()

	rt := &fakeDeliverer{delivery: runtime.Delivery{
		Action: &runtime.Action{
			Kind:       "message.send",
			TargetType: "group",
			TargetID:   "200",
			Text:       "reply text",
		},
	}}
	d.Register("action-plugin", rt, nil, nil)

	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(100 * time.Millisecond)

	sender.mu.Lock()
	count := len(sender.messages)
	sender.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 sent message, got %d", count)
	}
}
