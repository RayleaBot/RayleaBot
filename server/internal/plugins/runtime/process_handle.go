package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"sync"
	"time"
)

type ProcessSpec struct {
	PluginID             string
	InitTimeout          time.Duration
	InitMaxTotal         time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
}

type Handle struct {
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout *bufio.Reader
	Spec   ProcessSpec

	writeMu sync.Mutex
	done    chan struct{}
	exitMu  sync.RWMutex
	exitErr error
}

func NewHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec ProcessSpec) *Handle {
	return &Handle{
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Spec:   spec,
		done:   make(chan struct{}),
	}
}

func (h *Handle) Done() <-chan struct{} {
	if h == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return h.done
}

func (h *Handle) SetExit(err error) {
	h.exitMu.Lock()
	defer h.exitMu.Unlock()

	h.exitErr = err
	close(h.done)
}

func (h *Handle) ExitResult() (error, bool) {
	select {
	case <-h.done:
		h.exitMu.RLock()
		defer h.exitMu.RUnlock()
		return h.exitErr, true
	default:
		return nil, false
	}
}

func (h *Handle) Watch() {
	if h == nil || h.Cmd == nil {
		return
	}
	h.SetExit(h.Cmd.Wait())
}

func (h *Handle) WriteJSONLine(value any) error {
	if h == nil {
		return fmt.Errorf("plugin process handle is not available")
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	return writeJSONLine(h.Stdin, value)
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

func (m *Manager) watchRunningProcess(handle *Handle) {
	<-handle.Done()

	waitErr, _ := handle.ExitResult()

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
			fmt.Sprintf("插件%s运行时异常退出，累计崩溃 %d 次", pluginIDLabel(handle.Spec.PluginID), crashCount),
			"component", "runtime",
			"plugin_id", handle.Spec.PluginID,
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
		"插件"+pluginIDLabel(handle.Spec.PluginID)+"运行时已退出",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopped),
	)
}

func (m *Manager) reconcileExitedProcess(handle *Handle, waitErr error) {
	if waitErr != nil {
		m.markStopped(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		return
	}

	m.markStopped("", "", nil)
}

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

func (m *Manager) cleanupFailedStart(handle *Handle, code, message string, err error) {
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
