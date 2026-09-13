package rayleabot

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A0 only: the host session identifier is supplied by the test. Wire actions
// and public callback APIs remain gated on the session contract and D4.
type prototypeContinuations struct {
	mu      sync.Mutex
	entries map[string]func(*EventContext)
}

func (p *prototypeContinuations) install(id string, callback func(*EventContext)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = make(map[string]func(*EventContext))
	}
	p.entries[id] = callback
}
func (p *prototypeContinuations) take(id string) func(*EventContext) {
	p.mu.Lock()
	defer p.mu.Unlock()
	callback := p.entries[id]
	delete(p.entries, id)
	return callback
}

func TestConversationPrototypeCallbackReleasesConcurrencyOne(t *testing.T) {
	var continuations prototypeContinuations
	var previous *EventContext
	var turns atomic.Int32
	state := &runtimeState{
		location:  time.UTC,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		semaphore: make(chan struct{}, 1),
		client:    &runtimeClient{writer: jsonWriter{out: io.Discard}, done: make(chan struct{}), actionTimeout: time.Second},
	}
	var resume func(*EventContext)
	state.config.Store(&configSnapshot{values: map[string]any{}})
	resume = func(event *EventContext) {
		if previous != nil {
			if previous == event || previous.RequestID == event.RequestID {
				t.Error("resumed using old event ownership")
			}
			if err := previous.Actions().Call(context.Background(), "logger.write", map[string]any{}, nil); err == nil {
				t.Error("old context accepted an action")
			}
		}
		previous = event
		if turns.Add(1) < 4 {
			continuations.install("session", resume)
		}
	}
	state.handler = HandlerFunc(func(_ context.Context, event *EventContext) error {
		if event.RequestID == "start" {
			resume(event)
		} else if callback := continuations.take("session"); callback != nil {
			callback(event)
		}
		return nil // Real SDK automatic terminal response owns this new request.
	})
	for _, id := range []string{"start", "role", "operation", "confirm", "late"} {
		state.startEvent(context.Background(), id, Event{EventType: "message.group"})
		done := make(chan struct{})
		go func() { state.handlers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("callback retained SDK execution permit")
		}
		if len(state.semaphore) != 0 {
			t.Fatal("SDK permit leaked")
		}
	}
	if turns.Load() != 4 || len(continuations.entries) != 0 {
		t.Fatal("late event resumed callback or mapping leaked")
	}
}

func TestConversationPrototypeTimeoutAndSendFailureDeliverOnce(t *testing.T) {
	for _, failure := range []string{"send_failed", "notification_lost"} {
		var cache prototypeContinuations
		var calls atomic.Int32
		cache.install("session", func(*EventContext) { calls.Add(1) })
		deliver := func() {
			if callback := cache.take("session"); callback != nil {
				callback(nil)
			}
		}
		if failure == "notification_lost" {
			finished := make(chan struct{})
			timer := time.AfterFunc(time.Millisecond, func() { deliver(); close(finished) })
			t.Cleanup(func() { timer.Stop() })
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("local timeout depended on host notification")
			}
		} else {
			deliver()
		}
		var contenders sync.WaitGroup
		for range 16 {
			contenders.Go(deliver)
		}
		contenders.Wait()
		if calls.Load() != 1 || len(cache.entries) != 0 {
			t.Fatal("send/timeout/reply race delivered more than once")
		}
	}
}
