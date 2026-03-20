package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Summary struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	PluginID  string `json:"plugin_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type Stream struct {
	mu               sync.RWMutex
	history          []Summary
	limit            int
	nextSubscriberID uint64
	subscribers      map[uint64]chan Summary
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
	copy(cloned, s.history)
	return cloned
}

func (s *Stream) Append(summary Summary) {
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

	mu  sync.Mutex
	buf bytes.Buffer
}

func NewSummaryWriter(out io.Writer, stream *Stream) *SummaryWriter {
	return &SummaryWriter{
		out:    out,
		stream: stream,
	}
}

func (w *SummaryWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	if n <= 0 || w.stream == nil {
		return n, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = w.buf.Write(p[:n])
	for {
		buffered := w.buf.Bytes()
		index := bytes.IndexByte(buffered, '\n')
		if index < 0 {
			break
		}

		line := append([]byte(nil), buffered[:index+1]...)
		w.buf.Next(index + 1)
		if summary, ok := summaryFromJSONLine(line); ok {
			w.stream.Append(summary)
		}
	}

	return n, err
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
		Timestamp: toString(body["ts"]),
		Level:     strings.ToLower(toString(body["level"])),
		Source:    toString(body["component"]),
		Message:   toString(body["msg"]),
		PluginID:  toString(body["plugin_id"]),
		RequestID: toString(body["request_id"]),
	}

	if summary.Timestamp == "" || summary.Level == "" || summary.Message == "" {
		return Summary{}, false
	}
	if summary.Source == "" {
		summary.Source = "server"
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
