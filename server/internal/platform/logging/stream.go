package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
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

const (
	writeQueueCapacity = 1024
	writeBatchSize     = 256
	writeBatchDelay    = 100 * time.Millisecond
)

type writeRequest struct {
	summary Summary
	flushed chan struct{}
}

type Stream struct {
	admission        sync.RWMutex
	closed           bool
	writerStarted    bool
	writeQueue       chan writeRequest
	writerDone       chan struct{}
	closeOnce        sync.Once
	persistenceCtx   context.Context
	stopPersistence  context.CancelFunc
	mu               sync.RWMutex
	history          []Summary
	limit            int
	bootID           string
	hub              pubsub.Hub[Summary]
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

	persistenceCtx, stopPersistence := context.WithCancel(context.Background())
	return &Stream{
		persistenceCtx:  persistenceCtx,
		stopPersistence: stopPersistence,
		limit:           limit,
		writeQueue:      make(chan writeRequest, writeQueueCapacity),
		writerDone:      make(chan struct{}),
		clock:           time.Now,
		flushTicker:     5 * time.Second,
		flushNotify:     make(chan struct{}, 1),
		flushStop:       make(chan struct{}),
	}
}

func (s *Stream) Snapshot() []Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cloned := make([]Summary, len(s.history))
	for index, item := range s.history {
		cloned[index] = item
		cloned[index].Details = CloneMap(item.Details)
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

	s.mu.Lock()
	s.bootID = strings.TrimSpace(bootID)
	s.mu.Unlock()
}

func (s *Stream) Append(summary Summary) {
	s.appendNormalized(NormalizeSummary(summary))
}

// appendNormalized takes ownership of details already normalized by this package.
func (s *Stream) appendNormalized(summary Summary) {
	s.admission.RLock()
	defer s.admission.RUnlock()
	if s.closed {
		return
	}
	s.mu.RLock()
	bootID, repository, spool := s.bootID, s.repository, s.spool
	s.mu.RUnlock()
	if summary.BootID == "" {
		summary.BootID = bootID
	}
	if repository == nil {
		s.appendInMemory(summary)
		return
	}
	select {
	case s.writeQueue <- writeRequest{summary: summary}:
	default:
		s.appendToSpool(summary, spool, errors.New("management log write queue full"))
	}
}

func (s *Stream) appendToSpool(summary Summary, spool *SpoolQueue, cause error) {
	if spool == nil {
		s.reportPersistenceFailure("drop management log without spool: %v", cause)
		return
	}
	if err := spool.Append(summary); err != nil {
		s.reportPersistenceFailure("drop management log after persistence failed: cause=%v spool=%v", cause, err)
		return
	}
	s.appendInMemory(summary)
	s.signalFlush()
}

func (s *Stream) saveBatch(batch []Summary) {
	if len(batch) == 0 {
		return
	}
	s.mu.RLock()
	repository, spool := s.repository, s.spool
	s.mu.RUnlock()
	var err error
	if repository != nil {
		ctx, cancel := context.WithTimeout(s.persistenceCtx, 5*time.Second)
		err = repository.SaveSummaries(ctx, batch)
		cancel()
	}
	for _, summary := range batch {
		if err != nil {
			s.appendToSpool(summary, spool, err)
		} else {
			s.appendInMemory(summary)
		}
	}
	if err == nil && spool != nil && spool.HasEntries() {
		s.signalFlush()
	}
}

func (s *Stream) writeLoop() {
	defer close(s.writerDone)
	batch := make([]Summary, 0, writeBatchSize)
	timer := time.NewTimer(writeBatchDelay)
	timer.Stop()
	defer timer.Stop()
	var deadline <-chan time.Time
	flush := func() {
		timer.Stop()
		deadline = nil
		s.saveBatch(batch)
		clear(batch)
		batch = batch[:0]
	}
	for {
		select {
		case request, ok := <-s.writeQueue:
			if !ok {
				flush()
				return
			}
			if request.flushed != nil {
				flush()
				close(request.flushed)
				continue
			}
			if len(batch) == 0 {
				timer.Reset(writeBatchDelay)
				deadline = timer.C
			}
			batch = append(batch, request.summary)
			if len(batch) == writeBatchSize {
				flush()
			}
		case <-deadline:
			flush()
		}
	}
}

// Flush waits for previously queued summaries to finish their database or spool
// persistence attempt and publication. It does not replay the spool.
func (s *Stream) Flush(ctx context.Context) error {
	s.admission.RLock()
	if s.closed {
		s.admission.RUnlock()
		select {
		case <-s.writerDone:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if !s.writerStarted {
		s.admission.RUnlock()
		return nil
	}
	done := make(chan struct{})
	select {
	case s.writeQueue <- writeRequest{flushed: done}:
		s.admission.RUnlock()
	case <-ctx.Done():
		s.admission.RUnlock()
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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

	s.hub.PublishReplace(summary)
}

func (s *Stream) Subscribe(buffer int) (<-chan Summary, func()) {
	return s.hub.Subscribe(buffer)
}

func (s *Stream) SubscriberCount() int {
	return s.hub.SubscriberCount()
}

func (s *Stream) now() time.Time {
	if s.clock == nil {
		return time.Now()
	}
	return s.clock()
}

func (s *Stream) SetRepository(repository Repository, retentionDays int) {
	s.admission.Lock()
	defer s.admission.Unlock()
	if s.closed {
		return
	}
	if repository != nil && !s.writerStarted {
		s.writerStarted = true
		go s.writeLoop()
	}
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
	s.CloseContext(context.Background())
}

// CloseContext uses the application's remaining shutdown budget for database
// writes. Once it expires, pending batches fall back to spool before returning.
func (s *Stream) CloseContext(ctx context.Context) {
	stop := context.AfterFunc(ctx, s.stopPersistence)
	defer stop()
	s.closeOnce.Do(func() {
		defer s.stopPersistence()
		s.admission.Lock()
		s.closed = true
		if s.writerStarted {
			close(s.writeQueue)
		} else {
			close(s.writerDone)
		}
		s.mu.Lock()
		s.flushLoopClosed = true
		close(s.flushStop)
		s.mu.Unlock()
		s.admission.Unlock()
		<-s.writerDone
		s.flushWG.Wait()
		s.hub.Close()
	})
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

		ctx, cancel := context.WithTimeout(s.persistenceCtx, 5*time.Second)
		if err := s.flushSpool(ctx, false); err != nil {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		cancel()
	}
}

func (s *Stream) flushSpool(ctx context.Context, reportError bool) error {

	s.mu.RLock()
	repository := s.repository
	spool := s.spool
	s.mu.RUnlock()

	if repository == nil || spool == nil || !spool.HasEntries() {
		return nil
	}

	_, err := spool.Flush(ctx, repository)
	if err != nil {
		if reportError {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		return err
	}

	return nil
}

func (s *Stream) signalFlush() {
	select {
	case s.flushNotify <- struct{}{}:
	default:
	}
}

func (s *Stream) reportPersistenceFailure(format string, args ...any) {

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
