package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type processHandle struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	spec   Spec

	writeMu sync.Mutex
	done    chan struct{}
	exitMu  sync.RWMutex
	exitErr error
}

func newProcessHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec Spec) *processHandle {
	return &processHandle{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		spec:   spec,
		done:   make(chan struct{}),
	}
}

func (h *processHandle) setExit(err error) {
	h.exitMu.Lock()
	defer h.exitMu.Unlock()

	h.exitErr = err
	close(h.done)
}

func (h *processHandle) exitResult() (error, bool) {
	select {
	case <-h.done:
		h.exitMu.RLock()
		defer h.exitMu.RUnlock()
		return h.exitErr, true
	default:
		return nil, false
	}
}

func (m *Manager) cleanupFailedStart(handle *processHandle, code, message string, err error) {
	if handle != nil && handle.cmd != nil && handle.cmd.Process != nil {
		_ = handle.cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
	}
	m.markStopped(code, message, err)
}

func (m *Manager) cleanupFailedDelivery(handle *processHandle, code, message string, err error) {
	if handle != nil && handle.cmd != nil && handle.cmd.Process != nil {
		_ = handle.cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.done:
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
	cloned.Subscriptions = append([]string(nil), snapshot.Subscriptions...)
	return cloned
}

// ResetCrashCount resets the crash counter after a successful start.
func (m *Manager) ResetCrashCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.CrashCount = 0
	m.snap.NextRetryAt = nil
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
func (m *Manager) SetDeadLetterState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.State = StateDeadLetter
	m.snap.NextRetryAt = nil
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
}

func (m *Manager) watchRunningProcess(handle *processHandle) {
	<-handle.done

	waitErr, _ := handle.exitResult()

	m.mu.RLock()
	if m.proc != handle || m.snap.State != StateRunning {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	if waitErr != nil {
		m.mu.Lock()
		m.snap.CrashCount++
		crashCount := m.snap.CrashCount
		m.snap.State = StateCrashed
		now := m.deps.now()
		m.snap.StoppedAt = &now
		m.snap.LastErrorCode = codePluginInternalError
		m.snap.LastErrorMessage = "plugin exited unexpectedly"
		pluginID := m.snap.PluginID
		m.proc = nil
		m.mu.Unlock()

		m.logger.Warn(
			"plugin runtime crashed",
			"component", "runtime",
			"plugin_id", handle.spec.PluginID,
			"runtime_state", string(StateCrashed),
			"crash_count", crashCount,
			"err", waitErr.Error(),
		)

		if m.opts.OnCrash != nil {
			m.opts.OnCrash(pluginID, crashCount, codePluginInternalError)
		}
		return
	}

	m.markStopped("", "", nil)
	m.logger.Info(
		"plugin runtime exited",
		"component", "runtime",
		"plugin_id", handle.spec.PluginID,
		"runtime_state", string(StateStopped),
	)
}

func (m *Manager) reconcileExitedProcess(handle *processHandle, waitErr error) {
	if waitErr != nil {
		m.markStopped(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		return
	}

	m.markStopped("", "", nil)
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func (h *processHandle) writeJSONLine(value any) error {
	if h == nil {
		return fmt.Errorf("plugin process handle is not available")
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	return writeJSONLine(h.stdin, value)
}

func writeJSONLine(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !json.Valid(encoded) {
		return fmt.Errorf("protocol frame encoded invalid json")
	}

	data := append(encoded, '\n')
	for len(data) > 0 {
		written, writeErr := writer.Write(data)
		if written > 0 {
			data = data[written:]
		}
		if writeErr != nil {
			return writeErr
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}

	return nil
}
