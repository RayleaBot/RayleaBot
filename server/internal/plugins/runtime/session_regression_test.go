package runtime

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestProductionRequestIDsAreUniqueAcrossConcurrentManagers(t *testing.T) {
	const count = 10000
	ids := make(chan string, count)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			manager := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
			for range count / 8 {
				ids <- manager.deps.requestID()
			}
		})
	}
	workers.Wait()
	close(ids)
	seen := make(map[string]bool, count)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate request ID: %s", id)
		}
		seen[id] = true
	}
}

func TestDuplicateSessionRegistrationPreservesOriginal(t *testing.T) {
	manager := testManager()
	handle := &Handle{}
	manager.proc = handle
	manager.snap.State = StateRunning
	first, err := manager.registerEventSession(context.Background(), handle, "same", Event{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.registerEventSession(context.Background(), handle, "same", Event{}); err == nil {
		t.Fatal("duplicate event accepted")
	}
	if _, err := manager.registerPingRequest(handle, "same"); err == nil {
		t.Fatal("ping collided with event")
	}
	manager.mu.Lock()
	manager.completeEventLocked(first, Delivery{RequestID: "same"}, nil)
	manager.mu.Unlock()
	select {
	case <-first.done:
	case <-time.After(time.Second):
		t.Fatal("original session was lost")
	}
}

func TestCanceledEventIsNotReportedAsTimeout(t *testing.T) {
	manager := testManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := manager.DeliverEvent(ctx, Event{})
	assertRuntimeErrorCode(t, err, codePluginEventCanceled)
}

func TestConcurrentStartCannotReplaceLiveProcess(t *testing.T) {
	manager := testManagerWithRequestIDs(Options{})
	spec := helperSpec(t, "success", "")
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- manager.Start(context.Background(), spec, testInitPayload()) }()
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful starts = %d", successes)
	}
	if manager.Snapshot().State != StateRunning {
		t.Fatal("winning process is not running")
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
