package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type inertProcessInput struct{}

func (inertProcessInput) Write(data []byte) (int, error) { return len(data), nil }
func (inertProcessInput) Close() error                   { return nil }

func TestFailedRuntimeRetainsHandleUntilExitIsConfirmed(t *testing.T) {
	t.Parallel()
	manager := testManager()
	handle := NewHandle(nil, inertProcessInput{}, nil, ProcessSpec{PluginID: "fixture"})
	manager.proc = handle
	manager.snap.State = StateRunning
	manager.snap.PluginID = "fixture"
	t.Cleanup(func() {
		if _, exited := handle.ExitResult(); !exited {
			handle.SetExit(nil)
		}
	})
	err := manager.failRuntime(handle, codePluginProtocolViolation, "fixture failure", io.ErrUnexpectedEOF)
	if err == nil {
		t.Fatal("failure reported success")
	}
	manager.mu.RLock()
	owned := manager.proc == handle
	manager.mu.RUnlock()
	if !owned || manager.Snapshot().State != StateStopping {
		t.Fatal("unconfirmed process exit discarded runtime ownership")
	}
	manager.SetStopped()
	if manager.Snapshot().State != StateStopping {
		t.Fatal("snapshot reset hid a live process")
	}
	if err := manager.Start(context.Background(), helperSpec(t, "ready", ""), testInitPayload()); err == nil {
		t.Fatal("replacement started before old process exit")
	}
	handle.SetExit(errors.New("terminated"))
	deadline := time.Now().Add(time.Second)
	for manager.Snapshot().State != StateStopped && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := manager.Snapshot(); got.State != StateStopped || got.LastErrorCode != codePluginProtocolViolation {
		t.Fatalf("confirmed exit not reflected: %#v", got)
	}
	manager.mu.RLock()
	owned = manager.proc != nil
	manager.mu.RUnlock()
	if owned {
		t.Fatal("exited process ownership was not released")
	}
}

func TestRuntimeSnapshotResetCannotEraseCreation(t *testing.T) {
	t.Parallel()
	manager := testManager()
	manager.snap.State = StateStarting
	manager.SetStopped()
	if manager.Snapshot().State != StateStarting {
		t.Fatal("creation was mistaken for an already stopped process")
	}
}

func TestRetiredFailureCannotOverwriteReplacement(t *testing.T) {
	t.Parallel()
	manager := testManager()
	retired := NewHandle(nil, inertProcessInput{}, nil, ProcessSpec{PluginID: "fixture"})
	retired.SetExit(nil)
	current := NewHandle(nil, inertProcessInput{}, nil, ProcessSpec{PluginID: "fixture"})
	t.Cleanup(func() { current.SetExit(nil) })
	manager.proc = current
	manager.snap.State = StateRunning
	manager.finishFailedProcess(retired, errorf(codePluginProtocolViolation, "retired failure", nil))
	if manager.proc != current || manager.Snapshot().State != StateRunning {
		t.Fatal("old process completion overwrote replacement")
	}
}

func TestFatalRunningFailureNotifiesLifecycleAfterProcessReaping(t *testing.T) {
	t.Parallel()
	crashed := make(chan int, 2)
	manager := testManagerWithOptions(Options{OnCrash: func(_ string, count int, _ string) { crashed <- count }})
	spec := helperSpecWithConcurrency(t, "event-local-action-missing-parent-request-id", "", 2)
	if err := manager.Start(t.Context(), spec, testInitPayload()); err != nil {
		t.Fatal(err)
	}
	handle := manager.proc
	t.Cleanup(func() { _ = handle.Cmd.Process.Kill(); <-handle.Done() })
	_, err := manager.DeliverEvent(t.Context(), testRuntimeEvent())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
	select {
	case count := <-crashed:
		if count != 1 {
			t.Fatalf("crash count %d", count)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fatal running failure never reached lifecycle")
	}
	select {
	case <-handle.Done():
	default:
		t.Fatal("lifecycle was notified before failed process exit")
	}
	if state := manager.Snapshot().State; state != StateCrashed {
		t.Fatalf("failure state %s", state)
	}
	if err := manager.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-crashed:
		t.Fatal("failure generated duplicate lifecycle notification")
	default:
	}
}
