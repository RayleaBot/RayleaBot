package logging

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
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
		cloned[index].Details = logdetails.CloneMap(item.Details)
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

func (s *Stream) now() time.Time {
	if s == nil || s.clock == nil {
		return time.Now()
	}
	return s.clock()
}
