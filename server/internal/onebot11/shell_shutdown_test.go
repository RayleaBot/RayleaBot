package onebot11

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestShellStopWaitsForEventHandlerAndHonorsDeadline(t *testing.T) {
	shell := New("fixture", config.OneBotConfig{}, defaultAdapterConfig(), nil)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := shell.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	var calls atomic.Int32
	shell.SetEventHandler(func(ctx context.Context, _ chatevent.NormalizedEvent) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
	})
	shell.Start(t.Context())
	shell.eventQueue <- chatevent.NormalizedEvent{EventID: "first"}
	<-started
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if err := shell.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() = %v, want callback wait deadline", err)
	}
	<-cancelled
	// The old callback is still alive. A second Start must not create a new
	// dispatcher while the previous lifecycle has not actually stopped.
	shell.Start(t.Context())
	shell.eventQueue <- chatevent.NormalizedEvent{EventID: "queued-after-stop"}
	releaseOnce.Do(func() { close(release) })
	if err := shell.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("handled %d events, want only the in-flight event", got)
	}
}

func TestShellStopWaitsForReadyCallback(t *testing.T) {
	shell := New("fixture", config.OneBotConfig{}, defaultAdapterConfig(), nil)
	shell.Start(t.Context())
	finished := make(chan struct{})
	shell.startWorker(func(ctx context.Context) {
		<-ctx.Done()
		close(finished)
	})
	if err := shell.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	default:
		t.Fatal("Stop returned before its ready callback finished")
	}
}
