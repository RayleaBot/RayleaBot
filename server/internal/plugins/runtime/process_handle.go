package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwire"
)

type ProcessSpec struct {
	PluginID             string
	InitTimeout          time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
	IPCPendingActionsMax int
	IPCActionBurstCount  int
	IPCActionBurstWindow time.Duration
	IPCMessageMaxBytes   int
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
	// Output pipes and completion signals belong to this process generation.
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	protocolDone chan struct{}
	stderrDone   chan struct{}

	exitFailureReported bool           // guarded by the owning Manager.mu
	terminationObserved bool           // guarded by the owning Manager.mu
	terminationError    *plugins.Error // guarded by the owning Manager.mu
	failureRestart      bool           // guarded by the owning Manager.mu
}

func NewHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec ProcessSpec) *Handle {
	handle := &Handle{
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Spec:   spec,
		done:   make(chan struct{}),
	}
	if stdout != nil {
		handle.protocolDone = make(chan struct{})
	}
	return handle
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
	if h.stderrDone != nil {
		select {
		case <-h.stderrDone:
		case <-time.After(h.drainTimeout()):
			_ = h.stderr.Close()
		}
	}
}

func (h *Handle) drainTimeout() time.Duration {
	return max(h.Spec.ShutdownGrace, time.Second)
}

func (h *Handle) closeStdout() {
	if h.stdout != nil {
		_ = h.stdout.Close()
	}
}

// A descendant can inherit stdout after the plugin itself exits. Bound that
// drain by the shutdown budget, then release the owned reader to unblock it.
func (h *Handle) awaitProtocolDrain(ctx context.Context) error {
	if h.protocolDone == nil {
		return nil
	}
	drainCtx, cancel := context.WithTimeout(ctx, h.drainTimeout())
	defer cancel()
	select {
	case <-h.protocolDone:
		return nil
	case <-drainCtx.Done():
		h.closeStdout()
		return drainCtx.Err()
	}
}

func (h *Handle) WriteJSONLine(value any) error {
	if h == nil {
		return fmt.Errorf("plugin process handle is not available")
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	return writeJSONLineWithLimit(h.Stdin, value, h.Spec.IPCMessageMaxBytes)
}

func writeJSONLine(writer io.Writer, value any) error {
	return writeJSONLineWithLimit(writer, value, 0)
}

func writeJSONLineWithLimit(writer io.Writer, value any, maxBytes int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !json.Valid(encoded) {
		return fmt.Errorf("protocol frame encoded invalid json")
	}
	if maxBytes > 0 && len(encoded) > maxBytes {
		return fmt.Errorf("%w: encoded frame has %d bytes, limit %d", errProtocolFrameTooLarge, len(encoded), maxBytes)
	}

	if err := pluginwire.Validate(encoded, maxBytes); err != nil {
		return fmt.Errorf("invalid outgoing plugin frame: %w", err)
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
	_ = handle.awaitProtocolDrain(context.Background())

	waitErr, _ := handle.ExitResult()

	m.mu.Lock()
	if m.proc != handle || m.snap.State != StateRunning {
		m.mu.Unlock()
		return
	}

	if waitErr != nil {
		m.snap.CrashCount++
		crashCount := m.snap.CrashCount
		m.snap.State = StateCrashed
		runtimeErr := errorf(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		m.reportExitFailureLocked(handle, runtimeErr)
		m.abortPendingLocked(runtimeErr)
		now := m.deps.now()
		m.snap.StoppedAt = &now
		m.snap.LastErrorCode = codePluginInternalError
		m.snap.LastErrorMessage = "plugin exited unexpectedly"
		pluginID := m.snap.PluginID
		onCrash := m.opts.OnCrash
		m.proc = nil
		m.mu.Unlock()

		if onCrash != nil {
			onCrash(pluginID, crashCount, codePluginInternalError)
		}
		return
	}

	runtimeErr := errorf(codePluginInternalError, "plugin exited before delivery completed", nil)
	if len(m.pendingEvents)+len(m.pendingPings) > 0 {
		m.reportExitFailureLocked(handle, runtimeErr)
	}
	m.abortPendingLocked(runtimeErr)
	m.markStoppedLocked("", "", nil)
	exitReported := handle.exitFailureReported
	m.mu.Unlock()
	log := m.logger.Info
	if exitReported {
		log = m.logger.Debug
	}
	log(
		"插件"+pluginIDLabel(handle.Spec.PluginID)+"已退出",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopped),
	)
}

func (m *Manager) reconcileExitedProcess(handle *Handle, waitErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != handle {
		return
	}
	if waitErr != nil {
		m.markStoppedLocked(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		return
	}

	m.markStoppedLocked("", "", nil)
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
	_ = m.failRuntime(handle, code, message, err)
}

func (m *Manager) markStopped(code, message string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.markStoppedLocked(code, message, err)
}

func (m *Manager) markStoppedLocked(code, message string, err error) {
	if m.proc != nil {
		if _, exited := m.proc.ExitResult(); !exited {
			m.snap.State = StateStopping
			return
		}
	}
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
	if m.proc != nil || m.snap.State == StateStarting || m.snap.State == StateStopping {
		return
	}
	now := m.deps.now()
	m.snap.State = StateStopped
	m.snap.StoppedAt = &now
	m.snap.NextRetryAt = nil
	m.snap.EnteredDeadLetterAt = nil
}
