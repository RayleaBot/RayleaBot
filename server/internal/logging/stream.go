package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type Summary struct {
	LogID     string         `json:"log_id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Protocol  string `json:"protocol,omitempty"`
	PluginID  string `json:"plugin_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Details   map[string]any `json:"-"`
}

type Stream struct {
	mu               sync.RWMutex
	history          []Summary
	limit            int
	nextSubscriberID uint64
	subscribers      map[uint64]chan Summary
	repository       Repository
	retentionDays    int
}

func NewStream(limit int) *Stream {
	if limit <= 0 {
		limit = 1
	}

	return &Stream{
		limit:       limit,
		subscribers: map[uint64]chan Summary{},
	}
}

func (s *Stream) Snapshot() []Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cloned := make([]Summary, len(s.history))
	for index, item := range s.history {
		cloned[index] = item
		cloned[index].Details = cloneDetailsMap(item.Details)
	}
	return cloned
}

func (s *Stream) Limit() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.limit
}

func (s *Stream) SetRepository(repository Repository, retentionDays int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.repository = repository
	s.retentionDays = retentionDays
}

func (s *Stream) Append(summary Summary) {
	summary = NormalizeSummary(summary)

	s.mu.RLock()
	repository := s.repository
	retentionDays := s.retentionDays
	s.mu.RUnlock()

	if repository == nil {
		s.appendInMemory(summary)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = repository.SaveSummary(ctx, summary)
	if retentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		_ = repository.PruneOlderThan(ctx, cutoff)
	}

	s.appendInMemory(summary)
}

func (s *Stream) appendInMemory(summary Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) == s.limit {
		copy(s.history, s.history[1:])
		s.history[len(s.history)-1] = summary
	} else {
		s.history = append(s.history, summary)
	}

	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- summary:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- summary:
			default:
			}
		}
	}
}

func (s *Stream) Subscribe(buffer int) (<-chan Summary, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Summary, buffer)

	s.mu.Lock()
	id := s.nextSubscriberID
	s.nextSubscriberID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		subscriber, ok := s.subscribers[id]
		if !ok {
			return
		}

		delete(s.subscribers, id)
		close(subscriber)
	}
}

func (s *Stream) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.subscribers)
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
		Details:   extractSummaryDetails(body),
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
