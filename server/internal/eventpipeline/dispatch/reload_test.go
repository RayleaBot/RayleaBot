package dispatch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestRegisterReplacesExistingPluginRuntime(t *testing.T) {
	sender := &fakeSender{}
	d := New(slog.Default(), sender, nil, 16)
	defer d.Close()

	oldStarted := make(chan pluginruntime.Event, 1)
	newStarted := make(chan pluginruntime.Event, 1)
	oldRT := &fakeDeliverer{
		delivery: pluginruntime.Delivery{Result: map[string]any{"version": "old"}},
		started:  oldStarted,
	}
	newRT := &fakeDeliverer{
		delivery: pluginruntime.Delivery{Result: map[string]any{"version": "new"}},
		started:  newStarted,
	}

	d.Register("test-plugin", oldRT, []string{"message.group"}, nil, 1)

	// Verify old runtime receives events.
	d.Dispatch(context.Background(), testEvent(), "")
	select {
	case <-oldStarted:
	case <-time.After(time.Second):
		t.Fatal("old runtime did not receive the first event")
	}
	if oldRT.eventCount() != 1 {
		t.Fatalf("old runtime should have 1 event, got %d", oldRT.eventCount())
	}

	// Registering the same plugin ID replaces its active runtime.
	d.Register("test-plugin", newRT, []string{"message.group"}, nil, 1)

	// New events should go to new runtime.
	d.Dispatch(context.Background(), testEvent(), "")
	select {
	case <-newStarted:
	case <-time.After(time.Second):
		t.Fatal("new runtime did not receive the event after replacement")
	}
	if newRT.eventCount() != 1 {
		t.Fatalf("new runtime should have 1 event, got %d", newRT.eventCount())
	}
	// Old runtime should not receive the second event.
	if oldRT.eventCount() != 1 {
		t.Fatalf("old runtime should still have 1 event, got %d", oldRT.eventCount())
	}
	select {
	case <-oldStarted:
		t.Fatal("old runtime received an event after replacement")
	default:
	}
}
