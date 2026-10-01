package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/console"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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
	if options.RuntimeConfig == nil {
		// Without a configuration source the schema defaults apply.
		options.RuntimeConfig = func() config.RuntimeConfig { return config.RuntimeConfig{} }
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

func (m *Manager) ProcessDone() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proc == nil {
		return nil
	}
	return m.proc.Done()
}

func (m *Manager) cleanupComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proc == nil && m.snap.State != StateStarting && m.snap.State != StateStopping
}

func (m *Manager) abortPendingLocked(runtimeErr *plugins.Error) {
	for requestID, session := range m.pendingEvents {
		if session.completed {
			delete(m.pendingEvents, requestID)
			continue
		}
		var err error = runtimeErr
		if session.event.EventType == "plugin.request" {
			// The caller must see a lost provider as an unavailable service. It
			// cannot infer that from the runtime state, which is updated only
			// after the exiting process has been reaped.
			err = errorf(codePluginServiceUnavailable, "service provider stopped or changed runtime", runtimeErr)
		}
		m.closeSessionLocked(session, plugins.Delivery{}, err)
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

func (m *Manager) signalPendingRequests(handle *Handle, runtimeErr *plugins.Error) {
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

func (m *Manager) reportExitFailureLocked(handle *Handle, runtimeErr *plugins.Error) {
	if !handle.exitFailureReported {
		m.logger.Warn("插件意外退出或断开连接",
			"component", "runtime", "plugin_id", handle.Spec.PluginID,
			"runtime_state", string(m.snap.State), "crash_count", m.snap.CrashCount,
			"error_code", runtimeErr.Code, "err", runtimeErr.Error())
		handle.exitFailureReported = true
	}
	runtimeErr.MarkFailureReported()
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (m *Manager) ReadyForEvents() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snap.State == StateRunning
}

// ShutdownGrace returns the grace owned by the current process generation.
func (m *Manager) ShutdownGrace() time.Duration {
	if cfg := m.opts.RuntimeConfig(); cfg.ShutdownGraceSeconds > 0 {
		return cfg.PluginShutdownGrace()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proc != nil && m.proc.Spec.ShutdownGrace > 0 {
		return m.proc.Spec.ShutdownGrace
	}
	return m.opts.RuntimeConfig().PluginShutdownGrace()
}
