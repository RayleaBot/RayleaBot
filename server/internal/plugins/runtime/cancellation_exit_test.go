package runtime

import (
	"bufio"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestInitCancellationKeepsCancellationIdentity(t *testing.T) {
	manager := testManager()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	handle := NewHandle(nil, inertProcessInput{}, bufio.NewReader(reader), ProcessSpec{PluginID: "fixture", InitTimeout: time.Second})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := manager.awaitInitAck(ctx, handle, "init")
	if err != context.Canceled {
		t.Fatalf("cancellation became init failure: %v", err)
	}
}
func TestCanceledStartDoesNotPublishFailure(t *testing.T) {
	manager := testManager()
	spec := helperSpecWithTimings(t, "init-timeout", "", time.Second, time.Second, time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- manager.Start(ctx, spec, testInitPayload()) }()
	deadline := time.Now().Add(time.Second)
	for manager.ProcessDone() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("start error: %v", err)
	}
	if snap := manager.Snapshot(); snap.State != StateStopped || snap.LastErrorCode != "" {
		t.Fatalf("cancelled start: %#v", snap)
	}
}
func TestUnrequestedZeroExitNotifiesCrashLifecycle(t *testing.T) {
	crashes := make(chan int, 2)
	manager := testManagerWithOptions(Options{OnCrash: func(_ string, count int, _ string) { crashes <- count }})
	if err := manager.Start(t.Context(), helperSpec(t, "exit-after-ready", ""), testInitPayload()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })
	select {
	case count := <-crashes:
		if count != 1 {
			t.Fatal(count)
		}
	case <-time.After(time.Second):
		t.Fatal("zero exit never notified lifecycle")
	}
	if snap := manager.Snapshot(); snap.State != StateCrashed || snap.LastErrorCode != codePluginInternalError {
		t.Fatalf("unexpected exit: %#v", snap)
	}
	if err := manager.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-crashes:
		t.Fatal("duplicate crash")
	default:
	}
}
