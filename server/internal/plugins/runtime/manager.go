package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

type Manager struct {
	logger *slog.Logger
	deps   managerDeps
	opts   Options

	mu            sync.RWMutex
	protocolMu    sync.Mutex
	lifecycleGate chan struct{}
	proc          *Handle
	snap          Snapshot
	pendingEvents map[string]*eventSession
	pendingPings  map[string]*pingRequest
	expiredEvents map[string]time.Time

	pendingLocalActions int
	actionBurstStarted  time.Time
	actionBurstCount    int
}

func NewManager(logger *slog.Logger, options Options) *Manager {
	return newManager(logger, managerDeps{}, options)
}

var requestPrefix = rand.Text()
var requestSequence atomic.Uint64

func nextRuntimeRequestID() string {
	return fmt.Sprintf("req_%s_%d", requestPrefix, requestSequence.Add(1))
}

func (m *Manager) acquireLifecycle(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.lifecycleGate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newManager(logger *slog.Logger, deps managerDeps, options Options) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.requestID == nil {
		deps.requestID = nextRuntimeRequestID
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
		lifecycleGate: make(chan struct{}, 1),
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
		m.releaseSessionActionsLocked(session)
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
	if m.snap.State == StateStopping {
		runtimeErr = eventContextError(context.Canceled)
	} else if len(m.pendingEvents)+len(m.pendingPings) > 0 {
		m.reportExitFailureLocked(handle, runtimeErr)
	}
	m.abortPendingLocked(runtimeErr)
}

func (m *Manager) reportExitFailureLocked(handle *Handle, runtimeErr *Error) {
	if !handle.exitFailureReported {
		m.logger.Warn("插件"+pluginIDLabel(handle.Spec.PluginID)+"意外退出或断开连接。",
			"component", "runtime", "plugin_id", handle.Spec.PluginID,
			"runtime_state", string(m.snap.State), "crash_count", m.snap.CrashCount,
			"error_code", runtimeErr.Code, "err", runtimeErr.Error())
		handle.exitFailureReported = true
	}
	runtimeErr.failureReported = true
}
