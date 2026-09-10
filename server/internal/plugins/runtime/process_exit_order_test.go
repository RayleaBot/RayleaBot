package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestProcessExitWaitsForBufferedProtocolFrame(t *testing.T) {
	for _, observer := range []string{"watcher", "stop"} {
		for _, malformed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/malformed=%v", observer, malformed), func(t *testing.T) {
				manager := testManager()
				frame := map[string]any{"type": "result", "request_id": "last_frame", "status": "success", "data": map[string]any{"value": "last"}}
				if malformed {
					frame = map[string]any{"type": "action", "request_id": "last_frame", "action": "message.broadcast", "data": map[string]any{"text": "out of scope"}}
				}
				encoded, err := json.Marshal(frame)
				if err != nil {
					t.Fatal(err)
				}
				handle := NewHandle(nil, inertProcessInput{}, bufio.NewReader(strings.NewReader(string(encoded)+"\n")), ProcessSpec{PluginID: "fixture", ShutdownGrace: time.Second})
				manager.proc = handle
				manager.snap.State = StateRunning
				session, failure := manager.registerEventSession(context.Background(), handle, "last_frame", testRuntimeEvent())
				if failure != nil {
					t.Fatal(failure)
				}
				manager.protocolMu.Lock()
				readerDone := make(chan struct{})
				go func() { manager.readRuntimeFrames(handle); close(readerDone) }()
				handle.SetExit(nil)
				watcherDone := make(chan struct{})
				var stopErr error
				go func() {
					if observer == "stop" {
						ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
						defer cancel()
						stopErr = manager.Stop(ctx)
					} else {
						manager.watchRunningProcess(handle)
					}
					close(watcherDone)
				}()
				// Hold frame classification while process exit is already observable.
				// Exit must not replace that buffered frame with a generic EOF failure.
				select {
				case <-session.done:
					t.Errorf("process exit completed delivery before its buffered frame: %v", session.err)
				case <-time.After(100 * time.Millisecond):
				}
				manager.protocolMu.Unlock()
				for _, done := range []<-chan struct{}{readerDone, watcherDone, session.done} {
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						t.Fatal("frame/exit coordination did not finish")
					}
				}
				if stopErr != nil {
					t.Fatalf("stop after process exit: %v", stopErr)
				}
				if malformed {
					assertRuntimeErrorCode(t, session.err, codePluginProtocolViolation)
				} else if session.err != nil || session.delivery.Result["value"] != "last" {
					t.Fatalf("last valid result was lost: %#v, %v", session.delivery, session.err)
				}
			})
		}
	}
}

func TestNativeProcessFinalFramesSurviveImmediateExit(t *testing.T) {
	t.Run("result", func(t *testing.T) {
		manager := testManager()
		if err := manager.Start(t.Context(), helperSpec(t, "event-last-result-exit", ""), testInitPayload()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = manager.Stop(context.Background()) })
		manager.mu.RLock()
		handle := manager.proc
		manager.mu.RUnlock()
		delivery, err := manager.DeliverEvent(t.Context(), testRuntimeEvent())
		if err != nil || delivery.Result["value"] != "last" {
			t.Fatalf("last native result: %#v, %v", delivery, err)
		}
		select {
		case <-handle.Done():
		case <-time.After(runtimeTestDuration(time.Second)):
			t.Fatal("native process did not exit")
		}
		for range 2 {
			if err := manager.Stop(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if !manager.cleanupComplete() || manager.Snapshot().LastErrorCode != "" {
			t.Fatalf("clean exit was not reconciled: %#v", manager.Snapshot())
		}
	})
	t.Run("invalid_init", func(t *testing.T) {
		manager := testManager()
		err := manager.Start(t.Context(), helperSpec(t, "init-invalid-last-frame", ""), testInitPayload())
		assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
		if err := manager.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !manager.cleanupComplete() {
			t.Fatal("invalid init left an owned process")
		}
	})
}

func TestExitedProcessWithInheritedOutputHasBoundedDrain(t *testing.T) {
	manager := testManager()
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	handle := NewHandle(nil, inertProcessInput{}, bufio.NewReader(reader), ProcessSpec{PluginID: "fixture"})
	handle.stdout = reader
	manager.proc = handle
	manager.snap.State = StateRunning
	session, failure := manager.registerEventSession(t.Context(), handle, "inherited_output", testRuntimeEvent())
	if failure != nil {
		t.Fatal(failure)
	}
	readerDone := make(chan struct{})
	go func() { manager.readRuntimeFrames(handle); close(readerDone) }()
	handle.SetExit(nil)
	watchDone := make(chan struct{})
	go func() { manager.watchRunningProcess(handle); close(watchDone) }()
	for _, done := range []<-chan struct{}{watchDone, readerDone, session.done} {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("inherited output prevented process cleanup")
		}
	}
	assertRuntimeErrorCode(t, session.err, codePluginInternalError)
	if !manager.cleanupComplete() {
		t.Fatal("output drain retained the exited process")
	}
}
