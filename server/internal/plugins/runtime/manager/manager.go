package manager

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
)

type Manager struct {
	logger *slog.Logger
	deps   managerDeps
	opts   Options

	mu            sync.RWMutex
	protocolMu    sync.Mutex
	proc          *runtimeprocess.Handle
	snap          Snapshot
	pendingEvents map[string]*eventSession
	pendingPings  map[string]*pingRequest
	expiredEvents map[string]time.Time
}

func New(logger *slog.Logger, options Options) *Manager {
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
