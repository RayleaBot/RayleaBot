package chatpolicy

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

type blockedIngressMetadata struct {
	started  chan struct{}
	finished chan struct{}
	calls    atomic.Int32
}

func (m *blockedIngressMetadata) EnrichEventMetadata(ctx context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	m.calls.Add(1)
	close(m.started)
	<-ctx.Done()
	close(m.finished)
	return event
}

func TestIngressStopsAdmissionAndCancelsAcceptedWorkAtDrainDeadline(t *testing.T) {
	m := &blockedIngressMetadata{started: make(chan struct{}), finished: make(chan struct{})}
	s := NewIngress(IngressDeps{MetadataEnricher: m})
	done := make(chan struct{})
	go func() { s.HandleAdapterEvent(t.Context(), chatevent.NormalizedEvent{}); close(done) }()
	<-m.started
	s.StopAdmission()
	s.HandleAdapterEvent(t.Context(), chatevent.NormalizedEvent{})
	if m.calls.Load() != 1 {
		t.Fatal("late event admitted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := s.Drain(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drain = %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("accepted work not cancelled")
	}
	if err := s.Drain(t.Context()); err != nil {
		t.Fatal(err)
	}
}
