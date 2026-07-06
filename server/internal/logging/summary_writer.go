package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/redact"
)

const ProtocolOneBot11 = "onebot11"

type OneBotInboundMessageSummaryInput struct {
	SourceProtocol   string
	BotID            string
	EventType        string
	ConversationType string
	ConversationID   string
	SenderID         string
	TargetName       string
	ActorNickname    string
	PlainText        string
	PayloadFields    map[string]any
}

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

func normalizeSummaryTimestamp(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return trimmed
	}

	return parsed.UTC().Format(time.RFC3339Nano)
}

func IsSupportedProtocol(protocol string) bool {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
		return true
	default:
		return false
	}
}

func protocolFromSource(source string) string {
	switch strings.TrimSpace(source) {
	case "adapter", "adapter.onebot11", "bridge":
		return ProtocolOneBot11
	default:
		return ""
	}
}

func SourcesForProtocol(protocol string) []string {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
		return []string{"adapter", "adapter.onebot11", "bridge"}
	default:
		return nil
	}
}

func OneBotInboundMessageSummary(input OneBotInboundMessageSummaryInput) (string, bool) {
	if strings.TrimSpace(input.SourceProtocol) != ProtocolOneBot11 {
		return "", false
	}

	messageText := summarizeOneBotInboundMessageText(input.PlainText)
	if messageText == "" {
		return "", false
	}

	botID := strings.TrimSpace(input.BotID)
	if botID == "" {
		return "", false
	}

	senderID := strings.TrimSpace(input.SenderID)
	if senderID == "" {
		return "", false
	}

	senderDisplay := oneBotSenderDisplay(input)
	if senderDisplay == "" {
		senderDisplay = senderID
	}

	switch strings.TrimSpace(input.EventType) {
	case "message.group":
		return fmt.Sprintf("%s: %s%s%s(%s): %s",
			botID,
			oneBotGroupDisplay(input),
			oneBotSenderTitle(input.PayloadFields),
			senderDisplay,
			senderID,
			messageText,
		), true
	case "message.private":
		return fmt.Sprintf("%s: %s(%s): %s", botID, senderDisplay, senderID, messageText), true
	default:
		return "", false
	}
}

func summarizeOneBotInboundMessageText(text string) string {
	text = strings.TrimSpace(redact.SanitizeString(text))
	if text == "" {
		return ""
	}
	return redact.TruncateRunes(text, 160, "...")
}

func oneBotGroupDisplay(input OneBotInboundMessageSummaryInput) string {
	groupID := strings.TrimSpace(input.ConversationID)
	groupName := strings.TrimSpace(redact.SanitizeString(input.TargetName))
	if groupName == "" {
		return fmt.Sprintf("[%s]", groupID)
	}
	return fmt.Sprintf("[%s(%s)]", groupName, groupID)
}

func oneBotSenderTitle(payloadFields map[string]any) string {
	onebot := oneBotPayload(payloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		if title := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["title"]))); title != "" && title != "<nil>" {
			return fmt.Sprintf("[%s]", title)
		}
	}
	return ""
}

func oneBotSenderDisplay(input OneBotInboundMessageSummaryInput) string {
	onebot := oneBotPayload(input.PayloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		card := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["card"])))
		if card == "<nil>" {
			card = ""
		}
		nickname := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["nickname"])))
		if nickname == "<nil>" {
			nickname = ""
		}

		switch {
		case card != "" && nickname != "" && card != nickname:
			return card + "/" + nickname
		case card != "":
			return card
		case nickname != "":
			return nickname
		}
	}

	return strings.TrimSpace(redact.SanitizeString(input.ActorNickname))
}

func oneBotPayload(payloadFields map[string]any) map[string]any {
	if payloadFields == nil {
		return nil
	}
	onebot, _ := payloadFields["onebot"].(map[string]any)
	return onebot
}
