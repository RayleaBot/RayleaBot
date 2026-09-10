package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegistryStopsCurrentRetiredAndUnpublishedProcesses(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(nil, Options{})
	old := registry.GetOrCreate("fixture")
	next := registry.NewDetached()
	unpublished := registry.NewDetached()
	for _, manager := range []*Manager{old, next, unpublished} {
		spec := helperSpec(t, "success", "")
		spec.ShutdownGrace = 3 * time.Second
		if err := manager.Start(t.Context(), spec, testInitPayload()); err != nil {
			t.Fatal(err)
		}
		handle := manager.proc
		t.Cleanup(func() { _ = handle.Cmd.Process.Kill(); <-handle.Done() })
	}
	registry.Replace("fixture", next)
	registry.ReleaseRetired(old)
	registry.mu.RLock()
	_, held := registry.retired[old]
	registry.mu.RUnlock()
	if !held {
		t.Fatal("live old runtime was released before cleanup")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := registry.StopAll(ctx); err != nil {
		t.Fatal(err)
	}
	for _, manager := range []*Manager{old, next, unpublished} {
		if !manager.cleanupComplete() {
			t.Fatal("registry shutdown lost a process generation")
		}
	}
	registry.mu.RLock()
	remaining := len(registry.retired)
	registry.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("exited runtime references leaked: %d", remaining)
	}
}

func TestRegistryCancellationDoesNotReportCleanupSuccess(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(nil, Options{})
	manager := registry.GetOrCreate("fixture")
	handle := NewHandle(nil, inertProcessInput{}, nil, ProcessSpec{PluginID: "fixture"})
	manager.proc = handle
	manager.snap.State = StateRunning
	t.Cleanup(func() { handle.SetExit(nil) })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := registry.StopAll(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cleanup result: %v", err)
	}
	if manager.proc != handle {
		t.Fatal("cancelled cleanup dropped the process owner")
	}
}
