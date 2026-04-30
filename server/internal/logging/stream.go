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
	BootID    string         `json:"-"`
	LogID     string         `json:"log_id"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Source    string         `json:"source"`
	Message   string         `json:"message"`
	Protocol  string         `json:"protocol,omitempty"`
	PluginID  string         `json:"plugin_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"-"`
}

type Stream struct {
	mu               sync.RWMutex
	history          []Summary
	limit            int
	bootID           string
	nextSubscriberID uint64
	subscribers      map[uint64]chan Summary
	repository       Repository
	retentionDays    int
	spool            *SpoolQueue
	stderr           io.Writer
	clock            func() time.Time
	flushTicker      time.Duration
	flushNotify      chan struct{}
	flushStop        chan struct{}
	flushWG          sync.WaitGroup
	flushLoopStarted bool
	flushLoopClosed  bool
	diagnosticMu     sync.Mutex
	lastDiagnostic   time.Time
}

func NewStream(limit int) *Stream {
	if limit <= 0 {
		limit = 1
	}

	return &Stream{
		limit:       limit,
		subscribers: map[uint64]chan Summary{},
		clock:       time.Now,
		flushTicker: 5 * time.Second,
		flushNotify: make(chan struct{}, 1),
		flushStop:   make(chan struct{}),
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

func (s *Stream) BootID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bootID
}

func (s *Stream) SetBootID(bootID string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.bootID = strings.TrimSpace(bootID)
	s.mu.Unlock()
}

func (s *Stream) SetRepository(repository Repository, retentionDays int) {
	s.mu.Lock()
	s.repository = repository
	s.retentionDays = retentionDays
	spool := s.spool
	s.mu.Unlock()

	if repository != nil && spool != nil && spool.HasEntries() {
		s.signalFlush()
	}
}

func (s *Stream) ConfigureSpool(queue *SpoolQueue, stderr io.Writer) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.spool = queue
	if stderr != nil {
		s.stderr = stderr
	}
	startLoop := queue != nil && !s.flushLoopStarted && !s.flushLoopClosed
	if startLoop {
		s.flushLoopStarted = true
		s.flushWG.Add(1)
	}
	s.mu.Unlock()

	if startLoop {
		go s.flushLoop()
	}
	if queue != nil && queue.HasEntries() {
		s.signalFlush()
	}
}

func (s *Stream) FlushSpool(ctx context.Context) error {
	return s.flushSpool(ctx, true)
}

func (s *Stream) Close() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.flushLoopClosed {
		s.mu.Unlock()
		return
	}
	waitForFlushLoop := s.flushLoopStarted
	s.flushLoopClosed = true
	close(s.flushStop)
	s.mu.Unlock()

	if waitForFlushLoop {
		s.flushWG.Wait()
	}
}

func (s *Stream) Append(summary Summary) {
	summary = NormalizeSummary(summary)

	s.mu.RLock()
	bootID := s.bootID
	repository := s.repository
	retentionDays := s.retentionDays
	spool := s.spool
	s.mu.RUnlock()

	if summary.BootID == "" {
		summary.BootID = bootID
	}
	summary = NormalizeSummary(summary)

	if repository == nil {
		s.appendInMemory(summary)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	saveErr := repository.SaveSummary(ctx, summary)
	if saveErr == nil {
		if retentionDays > 0 {
			cutoff := s.now().AddDate(0, 0, -retentionDays)
			_ = repository.PruneOlderThan(ctx, cutoff)
		}
		s.appendInMemory(summary)
		if spool != nil && spool.HasEntries() {
			s.signalFlush()
		}
		return
	} else if spool != nil {
		if spoolErr := spool.Append(summary); spoolErr == nil {
			s.appendInMemory(summary)
			s.signalFlush()
			return
		} else {
			s.reportPersistenceFailure("drop management log after db and spool persistence failed: db=%v spool=%v", saveErr, spoolErr)
			return
		}
	}

	s.reportPersistenceFailure("drop management log after db persistence failed and no spool is configured: %v", saveErr)
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

func (s *Stream) flushLoop() {
	defer s.flushWG.Done()

	ticker := time.NewTicker(s.flushTicker)
	defer ticker.Stop()

	for {
		select {
		case <-s.flushStop:
			return
		case <-ticker.C:
		case <-s.flushNotify:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.flushSpool(ctx, false); err != nil {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		cancel()
	}
}

func (s *Stream) flushSpool(ctx context.Context, reportError bool) error {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	repository := s.repository
	retentionDays := s.retentionDays
	spool := s.spool
	s.mu.RUnlock()

	if repository == nil || spool == nil || !spool.HasEntries() {
		return nil
	}

	result, err := spool.Flush(ctx, repository)
	if err != nil {
		if reportError {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		return err
	}
	if result.Flushed > 0 && retentionDays > 0 {
		cutoff := s.now().AddDate(0, 0, -retentionDays)
		if pruneErr := repository.PruneOlderThan(ctx, cutoff); pruneErr != nil {
			if reportError {
				s.reportPersistenceFailure("management log prune after spool flush failed: %v", pruneErr)
			}
			return pruneErr
		}
	}
	return nil
}

func (s *Stream) signalFlush() {
	if s == nil {
		return
	}
	select {
	case s.flushNotify <- struct{}{}:
	default:
	}
}

func (s *Stream) now() time.Time {
	if s == nil || s.clock == nil {
		return time.Now()
	}
	return s.clock()
}

func (s *Stream) reportPersistenceFailure(format string, args ...any) {
	if s == nil {
		return
	}

	s.mu.RLock()
	stderr := s.stderr
	s.mu.RUnlock()
	if stderr == nil {
		return
	}

	now := s.now()
	s.diagnosticMu.Lock()
	if !s.lastDiagnostic.IsZero() && now.Sub(s.lastDiagnostic) < 10*time.Second {
		s.diagnosticMu.Unlock()
		return
	}
	s.lastDiagnostic = now
	s.diagnosticMu.Unlock()

	_, _ = fmt.Fprintf(stderr, "rayleabot logging persistence: "+format+"\n", args...)
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
