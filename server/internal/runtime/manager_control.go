package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

func (m *Manager) Stop(ctx context.Context) error {
	m.deliverMu.Lock()
	defer m.deliverMu.Unlock()

	m.mu.Lock()
	handle := m.proc
	if handle == nil {
		stoppedAt := m.deps.now()
		m.snap.State = StateStopped
		m.snap.StoppedAt = &stoppedAt
		m.mu.Unlock()
		return nil
	}
	if waitErr, exited := handle.exitResult(); exited {
		m.mu.Unlock()
		m.reconcileExitedProcess(handle, waitErr)
		return nil
	}
	m.snap.State = StateStopping
	m.mu.Unlock()

	m.logger.Info(
		"plugin runtime stopping",
		"component", "runtime",
		"plugin_id", handle.spec.PluginID,
		"runtime_state", string(StateStopping),
	)

	writeErr := writeJSONLine(handle.stdin, shutdownFrame{
		ProtocolVersion: "1",
		Type:            "shutdown",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.spec.PluginID,
		RequestID:       m.deps.requestID(),
		Reason:          "stop",
	})
	_ = handle.stdin.Close()

	if waitErr, exited := handle.exitResult(); exited {
		m.reconcileExitedProcess(handle, waitErr)
		return nil
	}

	stopCtx, cancel := context.WithTimeout(ctx, handle.spec.ShutdownGrace)
	defer cancel()

	select {
	case <-handle.done:
		waitErr, _ := handle.exitResult()
		if waitErr != nil {
			m.markStopped(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
			return errorf(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
		}
		if writeErr != nil && !isIgnorableShutdownWriteError(writeErr) {
			m.markStopped(codePluginInternalError, "write shutdown frame", writeErr)
			return errorf(codePluginInternalError, "write shutdown frame", writeErr)
		}
		m.markStopped("", "", nil)
		m.logger.Info(
			"plugin runtime stopped",
			"component", "runtime",
			"plugin_id", handle.spec.PluginID,
			"runtime_state", string(StateStopped),
		)
		return nil
	case <-stopCtx.Done():
		if handle.cmd.Process != nil {
			_ = handle.cmd.Process.Kill()
		}
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
		m.markStopped(codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
		return errorf(codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
	}
}

func isIgnorableShutdownWriteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(err.Error(), "broken pipe")
}

func classifyProtocolReadError(handle *processHandle, readErr error, exitMessage string, protocolMessage string) *Error {
	if waitErr, exited := handle.exitResult(); exited {
		if waitErr == nil {
			return errorf(codePluginInternalError, exitMessage, nil)
		}
		return errorf(codePluginInternalError, exitMessage, waitErr)
	}
	if isProcessPipeClosedError(readErr) {
		return errorf(codePluginInternalError, exitMessage, nil)
	}
	return errorf(codePluginProtocolViolation, protocolMessage, readErr)
}

func isProcessPipeClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "file already closed") || strings.Contains(message, "bad file descriptor")
}

// Ping sends a ping frame to the plugin and waits for a pong response.
// Returns an error if the runtime is not running, the plugin does not respond
// within the event timeout, or the plugin returns a protocol violation.
// A timeout causes the runtime to be stopped.
func (m *Manager) Ping(ctx context.Context) error {
	m.deliverMu.Lock()
	defer m.deliverMu.Unlock()

	m.mu.RLock()
	handle := m.proc
	snapshot := m.snap
	m.mu.RUnlock()

	if handle == nil || snapshot.State == StateStopped {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if snapshot.State == StateStopping {
		return errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if snapshot.State != StateRunning {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not ready for ping", nil)
	}

	requestID := m.deps.requestID()
	if err := writeJSONLine(handle.stdin, pingFrame{
		ProtocolVersion: "1",
		Type:            "ping",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.spec.PluginID,
		RequestID:       requestID,
	}); err != nil {
		return errorf(codePluginInternalError, "write ping frame", err)
	}

	return m.awaitPong(ctx, handle, requestID)
}

func (m *Manager) awaitPong(ctx context.Context, handle *processHandle, requestID string) error {
	readCh := make(chan []byte, 1)
	readErrCh := make(chan error, 1)

	go func() {
		line, err := handle.stdout.ReadBytes('\n')
		if err != nil {
			readErrCh <- err
			return
		}
		readCh <- line
	}()

	timeout := handle.spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case line := <-readCh:
		if err := m.parsePongResponse(line, handle.spec.PluginID, requestID); err != nil {
			var runtimeErr *Error
			if errors.As(err, &runtimeErr) {
				m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			}
			return err
		}
		return nil
	case readErr := <-readErrCh:
		return classifyProtocolReadError(handle, readErr, "plugin exited during ping", "read plugin pong response")
	case <-handle.done:
		waitErr, _ := handle.exitResult()
		if waitErr == nil {
			return errorf(codePluginInternalError, "plugin exited during ping", nil)
		}
		return errorf(codePluginInternalError, "plugin exited during ping", waitErr)
	case <-timer.C:
		runtimeErr := errorf(codePluginEventTimeout, "plugin pong response timed out", nil)
		m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	case <-ctx.Done():
		runtimeErr := errorf(codePluginEventTimeout, "plugin pong response timed out", ctx.Err())
		m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	}
}

func (m *Manager) parsePongResponse(line []byte, pluginID string, requestID string) error {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}
	if envelope.Type != "pong" {
		return errorf(codePluginProtocolViolation, "plugin returned unexpected frame type in response to ping", nil)
	}
	return nil
}
