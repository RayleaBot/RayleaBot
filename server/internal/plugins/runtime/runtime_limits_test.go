package runtime

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
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
