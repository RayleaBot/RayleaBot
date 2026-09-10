package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

func TestSwapRoutesNewEventsWhileDrainingAcceptedEvents(t *testing.T) {
	d := New(slog.Default(), nil, nil, 4)
	t.Cleanup(d.Close)
	old := &fakeDeliverer{started: make(chan chatevent.Event, 2), blockCh: make(chan struct{})}
	next := &fakeDeliverer{started: make(chan chatevent.Event, 1)}
	d.Register("fixture", old, nil, nil, 1)
	for range 2 {
		if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDelivered {
			t.Fatal(result)
		}
	}
	<-old.started
	retired := d.SwapPlugin("fixture", next, nil, nil, 1)
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDelivered {
		t.Fatal(result)
	}
	select {
	case <-next.started:
	case <-time.After(time.Second):
		t.Fatal("new runtime blocked by old deliveries")
	}
	close(old.blockCh)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := retired.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if old.eventCount() != 2 || next.eventCount() != 1 {
		t.Fatalf("delivery counts old=%d new=%d", old.eventCount(), next.eventCount())
	}
}

func TestSwapDrainCancellationReleasesOldDelivery(t *testing.T) {
	d := New(slog.Default(), nil, nil, 4)
	t.Cleanup(d.Close)
	old := &stoppingDeliverer{started: make(chan struct{})}
	d.Register("fixture", old, nil, nil, 1)
	d.DispatchToPlugin(t.Context(), "fixture", testEvent())
	<-old.started
	retired := d.SwapPlugin("fixture", &fakeDeliverer{}, nil, nil, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := retired.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("drain error = %v", err)
	}
	select {
	case <-retired.done:
	case <-time.After(time.Second):
		t.Fatal("old delivery did not stop after drain cancellation")
	}
	if !old.stopped.Load() {
		t.Fatal("old delivery context was not canceled")
	}
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDelivered {
		t.Fatalf("old drain canceled replacement: %+v", result)
	}
}
