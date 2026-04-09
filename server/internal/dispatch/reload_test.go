package dispatch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func TestReloadPluginSwapsRuntime(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	oldRT := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"version": "old"}}}
	newRT := &fakeDeliverer{delivery: runtime.Delivery{Result: map[string]any{"version": "new"}}}

	d.Register("test-plugin", oldRT, nil, nil, 1)

	// Verify old runtime receives events.
	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(50 * time.Millisecond)
	if oldRT.eventCount() != 1 {
		t.Fatalf("old runtime should have 1 event, got %d", oldRT.eventCount())
	}

	// Reload by directly registering the new runtime (simulating what
	// ReloadPlugin does after the new manager passes init_ack).
	d.Register("test-plugin", newRT, nil, nil, 1)

	// New events should go to new runtime.
	d.Dispatch(context.Background(), testEvent(), "")
	time.Sleep(50 * time.Millisecond)
	if newRT.eventCount() != 1 {
		t.Fatalf("new runtime should have 1 event, got %d", newRT.eventCount())
	}
	// Old runtime should not receive the second event.
	if oldRT.eventCount() != 1 {
		t.Fatalf("old runtime should still have 1 event, got %d", oldRT.eventCount())
	}
}
