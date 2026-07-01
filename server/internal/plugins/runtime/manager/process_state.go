package manager

import (
	"fmt"
	"math"
	"time"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
)

// DefaultMaxCrashRetries is the maximum number of consecutive crash-restart
// attempts before the runtime enters dead_letter state.
const DefaultMaxCrashRetries = 5

// CrashBackoff computes the next retry delay using capped exponential backoff.
//
//	delay = min(initialSeconds * 2^(crashCount-1), maxSeconds)
//
// crashCount must be >= 1. initialSeconds and maxSeconds are clamped to
// sensible minimums if they are zero or negative.
func CrashBackoff(crashCount, initialSeconds, maxSeconds int) time.Duration {
	if initialSeconds <= 0 {
		initialSeconds = 2
	}
	if maxSeconds <= 0 {
		maxSeconds = 60
	}
	if crashCount <= 0 {
		crashCount = 1
	}

	delay := float64(initialSeconds) * math.Pow(2, float64(crashCount-1))
	if delay > float64(maxSeconds) {
		delay = float64(maxSeconds)
	}

	return time.Duration(delay) * time.Second
}

func (m *Manager) cleanupFailedStart(handle *runtimeprocess.Handle, code, message string, err error) {
	if handle != nil && handle.Cmd != nil && handle.Cmd.Process != nil {
		_ = handle.Cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	m.markStopped(code, message, err)
}

func (m *Manager) markStopped(code, message string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.markStoppedLocked(code, message, err)
}

func (m *Manager) markStoppedLocked(code, message string, err error) {
	stoppedAt := m.deps.now()
	m.proc = nil
	m.snap.State = StateStopped
	m.snap.StoppedAt = &stoppedAt
	if code == "" {
		m.snap.LastErrorCode = ""
		m.snap.LastErrorMessage = ""
		return
	}

	m.snap.LastErrorCode = code
	if err != nil {
		m.snap.LastErrorMessage = fmt.Sprintf("%s: %v", message, err)
		return
	}
	m.snap.LastErrorMessage = message
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	if snapshot.StartedAt != nil {
		startedAt := *snapshot.StartedAt
		cloned.StartedAt = &startedAt
	}
	if snapshot.StoppedAt != nil {
		stoppedAt := *snapshot.StoppedAt
		cloned.StoppedAt = &stoppedAt
	}
	if snapshot.NextRetryAt != nil {
		nextRetryAt := *snapshot.NextRetryAt
		cloned.NextRetryAt = &nextRetryAt
	}
	if snapshot.EnteredDeadLetterAt != nil {
		enteredDeadLetterAt := *snapshot.EnteredDeadLetterAt
		cloned.EnteredDeadLetterAt = &enteredDeadLetterAt
	}
	cloned.Subscriptions = append([]string(nil), snapshot.Subscriptions...)
	return cloned
}

// ResetCrashCount resets the crash counter after a successful start.
func (m *Manager) ResetCrashCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.CrashCount = 0
	m.snap.NextRetryAt = nil
	m.snap.EnteredDeadLetterAt = nil
}

// SetBackoffState transitions the runtime snapshot to backoff with a
// scheduled next retry time. The lifecycle controller calls this after
// a crash to indicate the backoff wait period.
func (m *Manager) SetBackoffState(nextRetry time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.State = StateBackoff
	m.snap.NextRetryAt = &nextRetry
}

// SetDeadLetterState transitions the runtime snapshot to dead_letter,
// indicating that the maximum crash-backoff attempts have been exhausted.
// EnteredDeadLetterAt records the entry timestamp so management surfaces
// can show how long the plugin has been in dead_letter.
func (m *Manager) SetDeadLetterState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.State = StateDeadLetter
	m.snap.NextRetryAt = nil
	now := m.deps.now()
	m.snap.EnteredDeadLetterAt = &now
}

// SetOnCrash registers the crash callback after construction. This is
// used when the callback depends on objects that reference the manager
// itself (e.g. the lifecycle controller).
func (m *Manager) SetOnCrash(cb CrashCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opts.OnCrash = cb
}

// SetStopped transitions the runtime snapshot to stopped without
// attempting to stop a process. Used when the runtime is in a
// non-running state (crashed, backoff, dead_letter) and needs to
// be reset.
func (m *Manager) SetStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.deps.now()
	m.snap.State = StateStopped
	m.snap.StoppedAt = &now
	m.snap.NextRetryAt = nil
	m.snap.EnteredDeadLetterAt = nil
}
