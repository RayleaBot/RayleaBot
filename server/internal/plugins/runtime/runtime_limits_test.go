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

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
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

func TestRememberLocalActionIDReturnsWhenHistoryIsAllPending(t *testing.T) {
	t.Parallel()

	session := &eventSession{
		localActionIDs:   make(map[string]struct{}),
		pendingActionIDs: make(map[string]struct{}),
	}
	for index := 0; index < 4; index++ {
		requestID := string(rune('a' + index))
		rememberLocalActionID(session, requestID, 4)
		session.pendingActionIDs[requestID] = struct{}{}
	}

	done := make(chan struct{})
	go func() {
		rememberLocalActionID(session, "e", 4)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("remembering an action with an all-pending history did not return")
	}
	for _, requestID := range []string{"a", "b", "c", "d", "e"} {
		if _, ok := session.localActionIDs[requestID]; !ok {
			t.Fatalf("request ID %q was not remembered: %v", requestID, session.localActionOrder)
		}
	}
}

func TestLocalActionAdmissionRejectsPendingOverflow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	manager := newManager(slog.New(slog.NewTextHandler(io.Discard, nil)), managerDeps{
		now:       func() time.Time { return now },
		requestID: func() string { return "unused" },
	}, Options{})
	handle := &Handle{Spec: ProcessSpec{PluginID: "limit-plugin", EffectiveConcurrency: 1}}
	session := &eventSession{
		requestID:        "event-1",
		event:            chatevent.Event{EventID: "event-1"},
		localActionIDs:   make(map[string]struct{}),
		pendingActionIDs: make(map[string]struct{}),
	}
	manager.proc = handle
	manager.snap.State = StateRunning
	manager.pendingEvents[session.requestID] = session
	manager.pendingLocalActions = maxPendingLocalActions

	manager.mu.Lock()
	rejection, runtimeErr := manager.routeLocalActionFrameLocked(handle, decodeRuntimeFrame(t, localActionFrameBytes(t, "action-overflow", session.requestID)))
	manager.mu.Unlock()
	if runtimeErr != nil || rejection == nil || rejection.code != codePlatformRateLimited || rejection.requestID != "action-overflow" {
		t.Fatalf("unexpected pending rejection: rejection=%#v err=%v", rejection, runtimeErr)
	}
	if session.pendingLocalAction != 0 || manager.pendingLocalActions != maxPendingLocalActions {
		t.Fatalf("rejected action changed pending counts: session=%d manager=%d", session.pendingLocalAction, manager.pendingLocalActions)
	}
	if _, remembered := session.localActionIDs["action-overflow"]; remembered {
		t.Fatal("rejected action request ID was remembered")
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

func decodeRuntimeFrame(t *testing.T, line []byte) pluginwire.Frame {
	t.Helper()
	var frame pluginwire.Frame
	if err := json.Unmarshal(line, &frame); err != nil {
		t.Fatalf("decode runtime frame: %v", err)
	}
	return frame
}
