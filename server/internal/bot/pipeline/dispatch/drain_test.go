package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func TestDrainCompletesAcceptedEventsAndRejectsNewAdmission(t *testing.T) {
	d := New(nil, nil, nil, 1)
	t.Cleanup(d.Close)
	runtime := &fakeDeliverer{started: make(chan chatevent.Event, 2), blockCh: make(chan struct{})}
	d.Register("fixture", runtime, nil, nil, 1)
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDelivered {
		t.Fatal(result)
	}
	<-runtime.started
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDelivered {
		t.Fatal(result)
	}
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.Outcome != OutcomeDropped || result.ErrorCode != errorcodes.PlatformRateLimited {
		t.Fatalf("full queue result: %#v", result)
	}
	drain := d.DrainPlugin("fixture")
	if result := d.DispatchToPlugin(t.Context(), "fixture", testEvent()); result.ErrorCode != errorcodes.PluginStopping {
		t.Fatalf("closed queue reported overload: %#v", result)
	}
	close(runtime.blockCh)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := drain.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if runtime.eventCount() != 2 {
		t.Fatalf("accepted events lost: %d", runtime.eventCount())
	}
	if err := d.DrainAll(ctx); err != nil {
		t.Fatal(err)
	}
	if d.Register("late", &fakeDeliverer{}, nil, nil, 1) {
		t.Fatal("registration reopened a drained dispatcher")
	}
	if _, err := d.SwapPlugin("late", &fakeDeliverer{}, nil, nil, 1); !errors.Is(err, ErrClosed) {
		t.Fatalf("replacement reopened dispatcher: %v", err)
	}
}

func TestRegistrationDoesNotWaitForRetiredBlockedWorker(t *testing.T) {
	d := New(nil, nil, nil, 2)
	old := &fakeDeliverer{started: make(chan chatevent.Event, 1), blockCh: make(chan struct{})}
	d.Register("fixture", old, nil, nil, 1)
	d.DispatchToPlugin(t.Context(), "fixture", testEvent())
	<-old.started
	registered := make(chan bool, 1)
	go func() { registered <- d.Register("fixture", &fakeDeliverer{}, nil, nil, 1) }()
	select {
	case accepted := <-registered:
		if !accepted {
			t.Fatal("replacement rejected")
		}
	case <-time.After(time.Second):
		close(old.blockCh)
		t.Fatal("new runtime registration waited for old worker")
	}
	closed := make(chan struct{})
	go func() { d.Close(); close(closed) }()
	close(old.blockCh)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("dispatcher lost retired worker ownership")
	}
}
