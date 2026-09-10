package rayleabot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestRedactionSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/redaction.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Input, Output string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, vector := range vectors {
		if got := redact(vector.Input); got != vector.Output {
			t.Errorf("redaction differs: got %q want %q", got, vector.Output)
		}
	}
}

type partialWriter struct{ bytes.Buffer }

func (w *partialWriter) Write(p []byte) (int, error) {
	if len(p) > 3 {
		p = p[:3]
	}
	return w.Buffer.Write(p)
}

type stalledWriter struct{}

func (stalledWriter) Write([]byte) (int, error) { return 0, nil }

func TestProtocolWriterValidatesBeforeWriting(t *testing.T) {
	var output partialWriter
	writer := jsonWriter{out: &output}
	if err := writer.write(protocolFrame{Type: "result", RequestID: "r"}); err == nil || output.Len() != 0 {
		t.Fatalf("invalid result escaped: error=%v bytes=%d", err, output.Len())
	}
	if err := writer.write(protocolFrame{Type: "pong", RequestID: "p"}); err != nil {
		t.Fatal(err)
	}
	var frame protocolFrame
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &frame); err != nil || frame.RequestID != "p" {
		t.Fatalf("partial write dropped bytes: %v %s", err, output.Bytes())
	}
	blocked := jsonWriter{out: stalledWriter{}}
	if err := blocked.write(protocolFrame{Type: "pong", RequestID: "p"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("stalled writer: %v", err)
	}
}
