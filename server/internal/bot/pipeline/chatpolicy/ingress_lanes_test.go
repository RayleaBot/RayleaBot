package chatpolicy

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

type laneRecorder struct {
	mu       sync.Mutex
	gates    map[string]chan struct{}
	started  chan string
	finished []string
}

func newLaneRecorder(blocked ...string) *laneRecorder {
	recorder := &laneRecorder{gates: make(map[string]chan struct{}), started: make(chan string, 64)}
	for _, eventID := range blocked {
		recorder.gates[eventID] = make(chan struct{})
	}
	return recorder
}

func (r *laneRecorder) EnrichEventMetadata(ctx context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	r.started <- event.EventID
	r.mu.Lock()
	gate := r.gates[event.EventID]
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	r.mu.Lock()
	r.finished = append(r.finished, event.EventID)
	r.mu.Unlock()
	return event
}

func (r *laneRecorder) release(eventID string) { close(r.gates[eventID]) }

func (r *laneRecorder) awaitStart(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-r.started:
		if got != want {
			t.Fatalf("started %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("event %q did not start", want)
	}
}

func (r *laneRecorder) assertNoStart(t *testing.T) {
	t.Helper()
	select {
	case got := <-r.started:
		t.Fatalf("event %q started while its conversation was busy", got)
	case <-time.After(20 * time.Millisecond):
	}
}

func (r *laneRecorder) awaitFinished(t *testing.T, want []string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		r.mu.Lock()
		finished := slices.Clone(r.finished)
		r.mu.Unlock()
		if slices.Equal(finished, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("finished = %v, want %v", finished, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func laneEvent(eventID, conversationID string) chatevent.NormalizedEvent {
	return chatevent.NormalizedEvent{EventID: eventID, SourceAdapter: "bot", ConversationType: "group", ConversationID: conversationID}
}

func TestEnqueueAdapterEventKeepsOtherConversationsMovingAndOrdersEachLane(t *testing.T) {
	recorder := newLaneRecorder("a1")
	s := NewIngress(IngressDeps{MetadataEnricher: recorder})

	s.EnqueueAdapterEvent(t.Context(), laneEvent("a1", "A"))
	recorder.awaitStart(t, "a1")
	s.EnqueueAdapterEvent(t.Context(), laneEvent("a2", "A"))
	s.EnqueueAdapterEvent(t.Context(), laneEvent("b1", "B"))
	recorder.awaitStart(t, "b1")
	recorder.awaitFinished(t, []string{"b1"})
	recorder.assertNoStart(t)

	recorder.release("a1")
	recorder.awaitStart(t, "a2")
	recorder.awaitFinished(t, []string{"b1", "a1", "a2"})
}

func TestEnqueueAdapterEventDropsBeyondConversationCapacity(t *testing.T) {
	recorder := newLaneRecorder("busy")
	s := NewIngress(IngressDeps{MetadataEnricher: recorder})

	s.EnqueueAdapterEvent(t.Context(), laneEvent("busy", "A"))
	recorder.awaitStart(t, "busy")
	want := []string{"busy"}
	for index := range inboundLaneCapacity + 2 {
		eventID := strconv.Itoa(index)
		s.EnqueueAdapterEvent(t.Context(), laneEvent(eventID, "A"))
		if index < inboundLaneCapacity {
			want = append(want, eventID)
		}
	}
	s.EnqueueAdapterEvent(t.Context(), laneEvent("other", "B"))
	recorder.awaitStart(t, "other")

	recorder.release("busy")
	recorder.awaitFinished(t, append([]string{"other"}, want...))
}

func TestStopAdmissionDropsQueuedLaneEvents(t *testing.T) {
	recorder := newLaneRecorder("a1")
	s := NewIngress(IngressDeps{MetadataEnricher: recorder})

	s.EnqueueAdapterEvent(t.Context(), laneEvent("a1", "A"))
	recorder.awaitStart(t, "a1")
	s.EnqueueAdapterEvent(t.Context(), laneEvent("a2", "A"))
	s.StopAdmission()
	s.EnqueueAdapterEvent(t.Context(), laneEvent("b1", "B"))

	recorder.release("a1")
	if err := s.Drain(t.Context()); err != nil {
		t.Fatal(err)
	}
	recorder.awaitFinished(t, []string{"a1"})
	recorder.assertNoStart(t)
}
