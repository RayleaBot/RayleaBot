package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestSummaryWriterRedactsStructuredLogValues(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	var output bytes.Buffer
	writer := NewSummaryWriter(&output, stream, func(text string) string {
		return strings.ReplaceAll(text, "fixture-only-secret", "[REDACTED]")
	})

	line := `{"ts":"2026-03-20T10:00:00Z","level":"ERROR","component":"runtime","msg":"stderr leaked fixture-only-secret","token":"fixture-only-secret"}` + "\n"
	if _, err := writer.Write([]byte(line)); err != nil {
		t.Fatalf("write structured log line: %v", err)
	}

	raw := output.String()
	if strings.Contains(raw, "fixture-only-secret") {
		t.Fatalf("raw structured log output leaked secret: %s", raw)
	}
	if !strings.Contains(raw, "[REDACTED]") {
		t.Fatalf("expected redacted output, got %s", raw)
	}

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("unexpected summary count: got %d want %d", len(summaries), 1)
	}
	if strings.Contains(summaries[0].Message, "fixture-only-secret") {
		t.Fatalf("summary message leaked secret: %#v", summaries[0])
	}
}
