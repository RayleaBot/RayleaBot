package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
)

type SummaryWriter struct {
	out    io.Writer
	stream *Stream
	redact func(string) string

	mu  sync.Mutex
	buf bytes.Buffer
}

func NewSummaryWriter(out io.Writer, stream *Stream, redact func(string) string) *SummaryWriter {
	return &SummaryWriter{
		out:    out,
		stream: stream,
		redact: redact,
	}
}

func (w *SummaryWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = w.buf.Write(p)
	for {
		buffered := w.buf.Bytes()
		index := bytes.IndexByte(buffered, '\n')
		if index < 0 {
			break
		}

		line := append([]byte(nil), buffered[:index+1]...)
		w.buf.Next(index + 1)
		line = w.normalizeLine(line)
		if _, err := w.out.Write(line); err != nil {
			return len(p), err
		}
		if summary, ok := summaryFromJSONLine(line); ok {
			if w.stream != nil {
				w.stream.Append(summary)
			}
		}
	}

	return len(p), nil
}

func (w *SummaryWriter) normalizeLine(line []byte) []byte {
	if w.redact == nil {
		return line
	}

	if redacted, ok := redactJSONLine(line, w.redact); ok {
		return redacted
	}

	trimmed := strings.TrimRight(string(line), "\r\n")
	return append([]byte(w.redact(trimmed)), '\n')
}

func redactJSONLine(line []byte, redact func(string) string) ([]byte, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return line, false
	}

	var body any
	if err := json.Unmarshal(trimmed, &body); err != nil {
		return nil, false
	}

	redacted := redactJSONValue(body, redact)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil, false
	}

	return append(encoded, '\n'), true
}

func redactJSONValue(value any, redact func(string) string) any {
	switch typed := value.(type) {
	case string:
		return redact(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactJSONValue(typed[index], redact)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactJSONValue(inner, redact)
		}
		return result
	default:
		return value
	}
}

func summaryFromJSONLine(line []byte) (Summary, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return Summary{}, false
	}

	var body map[string]any
	if err := json.Unmarshal(line, &body); err != nil {
		return Summary{}, false
	}

	summary := Summary{
		LogID:     toString(body["log_id"]),
		Timestamp: toString(body["ts"]),
		Level:     strings.ToLower(toString(body["level"])),
		Source:    toString(body["component"]),
		Message:   toString(body["msg"]),
		PluginID:  toString(body["plugin_id"]),
		RequestID: toString(body["request_id"]),
		Details:   logdetails.ExtractSummary(body),
	}
	summary = NormalizeSummary(summary)

	if summary.Timestamp == "" || summary.Level == "" || summary.Message == "" {
		return Summary{}, false
	}

	return summary, true
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
