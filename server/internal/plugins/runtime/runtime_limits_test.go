package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestReadProtocolLineRejectsOversizedFrameWithoutNewline(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReaderSize(strings.NewReader(strings.Repeat("x", 4096)), 32)
	line, err := readProtocolLine(reader, 128)
	if !errors.Is(err, errProtocolFrameTooLarge) {
		t.Fatalf("readProtocolLine error = %v, want frame-too-large", err)
	}
	if len(line) != 0 {
		t.Fatalf("oversized frame retained %d bytes", len(line))
	}
}

func TestWriteJSONLineRejectsOversizedFrame(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeJSONLineWithLimit(&output, map[string]any{"payload": strings.Repeat("x", 256)}, 64)
	if !errors.Is(err, errProtocolFrameTooLarge) {
		t.Fatalf("writeJSONLineWithLimit error = %v, want frame-too-large", err)
	}
	if output.Len() != 0 {
		t.Fatalf("oversized frame wrote %d bytes", output.Len())
	}
}

func TestLocalActionAdmissionRejectsPendingAndBurstOverflow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	manager := newManager(slog.New(slog.NewTextHandler(io.Discard, nil)), managerDeps{
		now:       func() time.Time { return now },
		requestID: func() string { return "unused" },
	}, Options{})
	handle := &Handle{Spec: ProcessSpec{
		PluginID:             "limit-plugin",
		EffectiveConcurrency: 1,
		IPCPendingActionsMax: 1,
		IPCActionBurstCount:  1,
		IPCActionBurstWindow: time.Second,
	}}
	session := &eventSession{
		requestID:        "event-1",
		event:            Event{EventID: "event-1"},
		localActionIDs:   make(map[string]struct{}),
		pendingActionIDs: make(map[string]struct{}),
	}
	manager.proc = handle
	manager.snap.State = StateRunning
	manager.pendingEvents[session.requestID] = session
	manager.pendingLocalActions = 1

	pendingFrame := localActionFrameBytes(t, "action-pending", session.requestID)
	manager.mu.Lock()
	rejection, runtimeErr := manager.routeLocalActionFrameLocked(handle, pendingFrame)
	manager.mu.Unlock()
	if runtimeErr != nil || rejection == nil || rejection.details["limit_scope"] != "runtime.ipc_pending_actions_max" {
		t.Fatalf("unexpected pending rejection: rejection=%#v err=%v", rejection, runtimeErr)
	}

	manager.pendingLocalActions = 0
	manager.actionBurstStarted = now
	manager.actionBurstCount = 1
	burstFrame := localActionFrameBytes(t, "action-burst", session.requestID)
	manager.mu.Lock()
	rejection, runtimeErr = manager.routeLocalActionFrameLocked(handle, burstFrame)
	manager.mu.Unlock()
	if runtimeErr != nil || rejection == nil || rejection.details["limit_scope"] != "runtime.ipc_action_burst_limit" {
		t.Fatalf("unexpected burst rejection: rejection=%#v err=%v", rejection, runtimeErr)
	}
}

func TestRememberLocalActionIDKeepsBoundedHistory(t *testing.T) {
	t.Parallel()

	session := &eventSession{
		localActionIDs:   make(map[string]struct{}),
		pendingActionIDs: make(map[string]struct{}),
	}
	for index := 0; index < 20; index++ {
		rememberLocalActionID(session, string(rune('a'+index)), 4)
	}
	if len(session.localActionIDs) > 4 || len(session.localActionOrder) > 4 {
		t.Fatalf("local action history is unbounded: ids=%d order=%d", len(session.localActionIDs), len(session.localActionOrder))
	}
}

func localActionFrameBytes(t *testing.T, requestID, parentRequestID string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"type":              "action",
		"request_id":        requestID,
		"parent_request_id": parentRequestID,
		"action":            "logger.write",
		"data": map[string]any{
			"level":   "info",
			"message": "fixture",
		},
	})
	if err != nil {
		t.Fatalf("marshal local action frame: %v", err)
	}
	return payload
}
