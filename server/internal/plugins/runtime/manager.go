package runtime

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

type Manager struct {
	logger *slog.Logger
	deps   managerDeps
	opts   Options

	mu            sync.RWMutex
	protocolMu    sync.Mutex
	proc          *Handle
	snap          Snapshot
	pendingEvents map[string]*eventSession
	pendingPings  map[string]*pingRequest
	expiredEvents map[string]time.Time
}

func NewManager(logger *slog.Logger, options Options) *Manager {
	return newManager(logger, managerDeps{
		now: time.Now,
		requestID: func() string {
			return fmt.Sprintf("req_%d", time.Now().UnixNano())
		},
	}, options)
}

func newManager(logger *slog.Logger, deps managerDeps, options Options) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.requestID == nil {
		deps.requestID = func() string {
			return fmt.Sprintf("req_%d", time.Now().UnixNano())
		}
	}
	if options.Console == nil {
		options.Console = console.NewStream(1000, 2*1024*1024)
	}
	if options.RedactText == nil {
		options.RedactText = func(text string) string {
			return text
		}
	}

	return &Manager{
		logger:        logger,
		deps:          deps,
		opts:          options,
		pendingEvents: make(map[string]*eventSession),
		pendingPings:  make(map[string]*pingRequest),
		expiredEvents: make(map[string]time.Time),
		snap: Snapshot{
			State: StateStopped,
		},
	}
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSnapshot(m.snap)
}

func (m *Manager) abortPendingLocked(runtimeErr *Error) {
	for requestID, session := range m.pendingEvents {
		if session.completed {
			delete(m.pendingEvents, requestID)
			continue
		}
		session.completed = true
		session.err = runtimeErr
		session.cancel()
		close(session.done)
		delete(m.pendingEvents, requestID)
	}

	for requestID, ping := range m.pendingPings {
		if ping.completed {
			delete(m.pendingPings, requestID)
			continue
		}
		ping.completed = true
		ping.err = runtimeErr
		ping.done <- runtimeErr
		close(ping.done)
		delete(m.pendingPings, requestID)
	}
}

func (m *Manager) signalPendingRequests(handle *Handle, runtimeErr *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != handle {
		return
	}
	m.abortPendingLocked(runtimeErr)
}
