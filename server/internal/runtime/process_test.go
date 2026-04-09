package runtime

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestWriteJSONLineRetriesShortWrites(t *testing.T) {
	t.Parallel()

	writer := &chunkedWriteCloser{maxChunk: 3}
	frame := map[string]any{
		"type":       "ping",
		"plugin_id":  "builtin-help",
		"request_id": "req_runtime_ping_0001",
	}

	if err := writeJSONLine(writer, frame); err != nil {
		t.Fatalf("writeJSONLine returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(writer.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("unexpected line count: got %d want 1", len(lines))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("expected one complete json line, got %q: %v", lines[0], err)
	}
	if decoded["request_id"] != "req_runtime_ping_0001" {
		t.Fatalf("unexpected decoded payload: %#v", decoded)
	}
}

func TestProcessHandleWriteJSONLineSerializesConcurrentFrames(t *testing.T) {
	t.Parallel()

	writer := &chunkedWriteCloser{maxChunk: 2}
	handle := &processHandle{stdin: writer}

	const (
		writersPerBatch = 8
		framesPerWriter = 20
	)

	var wg sync.WaitGroup
	for worker := 0; worker < writersPerBatch; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for frameIndex := 0; frameIndex < framesPerWriter; frameIndex++ {
				err := handle.writeJSONLine(map[string]any{
					"type":       "result",
					"plugin_id":  "builtin-help",
					"request_id": "req_runtime_local_action",
					"worker":     worker,
					"frame":      frameIndex,
				})
				if err != nil {
					t.Errorf("handle.writeJSONLine returned error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(writer.String()), "\n")
	expectedLines := writersPerBatch * framesPerWriter
	if len(lines) != expectedLines {
		t.Fatalf("unexpected line count: got %d want %d", len(lines), expectedLines)
	}

	seen := make(map[string]struct{}, expectedLines)
	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("expected valid json line, got %q: %v", line, err)
		}

		key := decoded["request_id"].(string) + ":" + formatJSONNumber(decoded["worker"]) + ":" + formatJSONNumber(decoded["frame"])
		seen[key] = struct{}{}
	}
	if len(seen) != expectedLines {
		t.Fatalf("unexpected unique frame count: got %d want %d", len(seen), expectedLines)
	}
}

func TestWriteJSONLineRejectsInvalidEmbeddedJSON(t *testing.T) {
	t.Parallel()

	writer := &chunkedWriteCloser{maxChunk: 8}
	frame := map[string]any{
		"type": "event",
		"event": map[string]any{
			"raw_payload": json.RawMessage(`{"broken":}`),
		},
	}

	err := writeJSONLine(writer, frame)
	if err == nil {
		t.Fatal("expected writeJSONLine to reject invalid embedded json")
	}
	if got := writer.String(); got != "" {
		t.Fatalf("expected no bytes to be written, got %q", got)
	}
}

type chunkedWriteCloser struct {
	maxChunk int
	mu       sync.Mutex
	buffer   bytes.Buffer
}

func (w *chunkedWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	chunkSize := w.maxChunk
	if chunkSize <= 0 || chunkSize > len(p) {
		chunkSize = len(p)
	}

	return w.buffer.Write(p[:chunkSize])
}

func (w *chunkedWriteCloser) Close() error {
	return nil
}

func (w *chunkedWriteCloser) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func formatJSONNumber(value any) string {
	switch typed := value.(type) {
	case float64:
		return strconv.Itoa(int(typed))
	case json.Number:
		return typed.String()
	case string:
		return typed
	default:
		return ""
	}
}
