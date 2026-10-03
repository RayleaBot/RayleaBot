package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
)

const ProtocolOneBot11 = "onebot11"
const ProtocolQQOfficial = "qqofficial"

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

		line := buffered[:index+1]
		w.buf.Next(index + 1)
		line, body := w.normalizeLine(line)
		if _, err := w.out.Write(line); err != nil {
			return len(p), err
		}
		if summary, ok := summaryFromObject(body); ok {
			if w.stream != nil {
				w.stream.Append(summary)
			}
		}
	}

	return len(p), nil
}

func (w *SummaryWriter) normalizeLine(line []byte) ([]byte, map[string]any) {
	redactor := func(text string) string {
		if w.redact != nil {
			text = w.redact(text)
		}
		text = redact.SensitiveText(text)
		// Match encoding/json's replacement of invalid UTF-8 before the same
		// decoded object is used for both the output and the management view.
		if !utf8.ValidString(text) {
			text = string([]rune(text))
		}
		return text
	}
	if redacted, body, ok := normalizeJSONLine(line, redactor); ok {
		return redacted, body
	}

	trimmed := strings.TrimRight(string(line), "\r\n")
	return append([]byte(redactor(trimmed)), '\n'), nil
}

func normalizeJSONLine(line []byte, redact func(string) string) ([]byte, map[string]any, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return line, nil, false
	}

	var body any
	if err := json.Unmarshal(trimmed, &body); err != nil {
		return nil, nil, false
	}

	redacted := redactJSONValue(body, redact)
	if object, ok := redacted.(map[string]any); ok && protocolFromSource(toString(object["component"])) == ProtocolOneBot11 {
		redacted = compactOneBot11LogDetails(object)
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil, nil, false
	}

	object, _ := redacted.(map[string]any)
	return append(encoded, '\n'), object, true
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
			if isSensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redactJSONValue(inner, redact)
			}
		}
		return result
	default:
		return value
	}
}

func summaryFromObject(body map[string]any) (Summary, bool) {
	if body == nil {
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
		Details:   ExtractSummary(body),
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

func NormalizeSummary(summary Summary) Summary {
	summary.BootID = strings.TrimSpace(summary.BootID)
	summary.LogID = strings.TrimSpace(summary.LogID)
	summary.Timestamp = normalizeSummaryTimestamp(summary.Timestamp)
	summary.Level = strings.ToLower(strings.TrimSpace(summary.Level))
	summary.Source = strings.TrimSpace(summary.Source)
	summary.Message = strings.TrimSpace(summary.Message)
	summary.PluginID = strings.TrimSpace(summary.PluginID)
	summary.RequestID = strings.TrimSpace(summary.RequestID)
	summary.Protocol = strings.TrimSpace(summary.Protocol)

	if summary.LogID == "" {
		summary.LogID = generateLogID()
	}

	if summary.Source == "" {
		summary.Source = "server"
	}
	if summary.Protocol == "" {
		summary.Protocol = protocolFromSource(summary.Source)
	}
	if summary.Protocol == ProtocolOneBot11 {
		summary.Message = strings.TrimSpace(redact.SanitizeString(summary.Message))
	}
	summary.Details = NormalizeProtocol(summary.Protocol, summary.Details)

	return summary
}

func FormatTimestamp(instant time.Time) string {
	return instant.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

func normalizeSummaryTimestamp(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return trimmed
	}

	return FormatTimestamp(parsed)
}

func IsSupportedProtocol(protocol string) bool {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11, ProtocolQQOfficial:
		return true
	default:
		return false
	}
}

func protocolFromSource(source string) string {
	switch strings.TrimSpace(source) {
	case "adapter", "adapter.onebot11", "bridge", "bridge.onebot11":
		return ProtocolOneBot11
	case "adapter.qqofficial", "bridge.qqofficial":
		return ProtocolQQOfficial
	default:
		return ""
	}
}

func SourcesForProtocol(protocol string) []string {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
		return []string{"adapter", "adapter.onebot11", "bridge", "bridge.onebot11"}
	case ProtocolQQOfficial:
		return []string{"adapter.qqofficial", "bridge.qqofficial"}
	default:
		return nil
	}
}
