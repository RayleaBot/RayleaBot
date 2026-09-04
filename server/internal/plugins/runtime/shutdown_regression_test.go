package runtime

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type observedProcessInput struct {
	io.WriteCloser
	started chan struct{}
	once    sync.Once
}

func (input *observedProcessInput) Write(data []byte) (int, error) {
	input.once.Do(func() { close(input.started) })
	return input.WriteCloser.Write(data)
}

func TestStopInterruptsBlockedOSPipe(t *testing.T) {
	manager := testManager()
	if err := manager.Start(t.Context(), helperSpec(t, "stdin-blocked", ""), testInitPayload()); err != nil {
		t.Fatal(err)
	}
	handle := manager.proc
	t.Cleanup(func() { _ = handle.Cmd.Process.Kill(); <-handle.Done() })
	input := &observedProcessInput{WriteCloser: handle.Stdin, started: make(chan struct{})}
	handle.Stdin = input
	written := make(chan error, 1)
	go func() {
		written <- handle.WriteJSONLine(map[string]any{"type": "result", "data": strings.Repeat("x", 1024*1024)})
	}()
	<-input.started
	ctx, cancel := context.WithTimeout(t.Context(), runtimeTestDuration(100*time.Millisecond))
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Stop(ctx) }()
	select {
	case err := <-done:
		assertRuntimeErrorCode(t, err, codePluginShutdownTimeout)
	case <-time.After(runtimeTestDuration(3 * time.Second)):
		t.Fatal("stop could not release the OS pipe")
	}
	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("pipe writer survived runtime stop")
	}
	select {
	case <-handle.Done():
	default:
		t.Fatal("plugin process survived runtime stop")
	}
}

type blockedProcessInput struct {
	started   chan struct{}
	closed    chan struct{}
	writeOnce sync.Once
	closeOnce sync.Once
	onClose   func()
}

func (input *blockedProcessInput) Write([]byte) (int, error) {
	input.writeOnce.Do(func() { close(input.started) })
	<-input.closed
	return 0, io.ErrClosedPipe
}

func (input *blockedProcessInput) Close() error {
	input.closeOnce.Do(func() {
		close(input.closed)
		input.onClose()
	})
	return nil
}

func TestStopAllTerminatesEveryRuntimeWithBlockedInput(t *testing.T) {
	registry := NewRegistry(nil, Options{})
	var managers []*Manager
	var writes []<-chan error
	for _, id := range []string{"blocked-a", "blocked-b"} {
		manager := registry.GetOrCreate(id)
		input := &blockedProcessInput{started: make(chan struct{}), closed: make(chan struct{})}
		handle := NewHandle(nil, input, nil, ProcessSpec{PluginID: id, ShutdownGrace: time.Second})
		input.onClose = func() { handle.SetExit(nil) }
		t.Cleanup(func() { _ = input.Close() })
		manager.proc = handle
		manager.snap.State = StateRunning
		managers = append(managers, manager)
		// A response may still own the pipe after its event has expired.
		written := make(chan error, 1)
		go func() { written <- handle.WriteJSONLine(map[string]any{"type": "result"}) }()
		<-input.started
		writes = append(writes, written)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- registry.StopAll(ctx) }()
	select {
	case err := <-done:
		assertRuntimeErrorCode(t, err, codePluginShutdownTimeout)
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not interrupt the blocked pipe writes")
	}
	for index, manager := range managers {
		if manager.Snapshot().State != StateStopped {
			t.Errorf("runtime %d survived the shared shutdown deadline", index)
		}
		select {
		case <-writes[index]:
		case <-time.After(time.Second):
			t.Fatal("runtime writer was leaked")
		}
	}
}
