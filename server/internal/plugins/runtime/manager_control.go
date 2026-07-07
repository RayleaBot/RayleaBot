package runtime

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	handle := m.proc
	if handle == nil {
		stoppedAt := m.deps.now()
		m.snap.State = StateStopped
		m.snap.StoppedAt = &stoppedAt
		m.mu.Unlock()
		return nil
	}
	if waitErr, exited := handle.ExitResult(); exited {
		m.mu.Unlock()
		m.reconcileExitedProcess(handle, waitErr)
		return nil
	}
	m.snap.State = StateStopping
	m.mu.Unlock()

	for {
		m.mu.RLock()
		activeSessions := len(m.pendingEvents)
		m.mu.RUnlock()
		if activeSessions == 0 {
			break
		}
		if ctx.Err() != nil {
			return m.failRuntime(handle, codePluginShutdownTimeout, "plugin shutdown timed out", ctx.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}

	m.logger.Info(
		"插件"+pluginIDLabel(handle.Spec.PluginID)+"运行时正在停止",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopping),
	)

	writeErr := handle.WriteJSONLine(ShutdownFrame{
		ProtocolVersion: "1",
		Type:            "shutdown",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.Spec.PluginID,
		RequestID:       m.deps.requestID(),
		Reason:          "stop",
	})
	_ = handle.Stdin.Close()

	if writeErr != nil && !isIgnorableShutdownWriteError(writeErr) {
		return m.failRuntime(handle, codePluginInternalError, "write shutdown frame", writeErr)
	}

	stopCtx, cancel := context.WithTimeout(ctx, handle.Spec.ShutdownGrace)
	defer cancel()

	select {
	case <-handle.Done():
		waitErr, _ := handle.ExitResult()
		if waitErr != nil {
			m.markStopped(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
			return errorf(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
		}
		m.markStopped("", "", nil)
		m.logger.Info(
			"插件"+pluginIDLabel(handle.Spec.PluginID)+"运行时已停止",
			"component", "runtime",
			"plugin_id", handle.Spec.PluginID,
			"runtime_state", string(StateStopped),
		)
		return nil
	case <-stopCtx.Done():
		return m.failRuntime(handle, codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
	}
}

func isIgnorableShutdownWriteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "broken pipe") || strings.Contains(message, "pipe is being closed")
}

func classifyProtocolReadError(handle *Handle, readErr error, exitMessage string, protocolMessage string) *Error {
	if waitErr, exited := handle.ExitResult(); exited {
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

func (m *Manager) Ping(ctx context.Context) error {
	m.mu.RLock()
	handle := m.proc
	m.mu.RUnlock()
	if handle == nil {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}

	requestID := m.deps.requestID()
	request, runtimeErr := m.registerPingRequest(handle, requestID)
	if runtimeErr != nil {
		return runtimeErr
	}

	if err := handle.WriteJSONLine(PingFrame{
		ProtocolVersion: "1",
		Type:            "ping",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.Spec.PluginID,
		RequestID:       requestID,
	}); err != nil {
		m.mu.Lock()
		delete(m.pendingPings, requestID)
		m.mu.Unlock()
		return m.failRuntime(handle, codePluginInternalError, "write ping frame", err)
	}

	timeout := handle.Spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-request.done:
		return err
	case <-timer.C:
		return m.failRuntime(handle, codePluginEventTimeout, "plugin pong response timed out", nil)
	case <-ctx.Done():
		return m.failRuntime(handle, codePluginEventTimeout, "plugin pong response timed out", ctx.Err())
	}
}

func (m *Manager) registerPingRequest(handle *Handle, requestID string) (*pingRequest, *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle || handle == nil {
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if m.snap.State == StateStopping {
		return nil, errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if m.snap.State != StateRunning {
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not ready for ping", nil)
	}

	request := &pingRequest{done: make(chan error, 1)}
	m.pendingPings[requestID] = request
	return request, nil
}

func (m *Manager) completePingLocked(requestID string, request *pingRequest, err error) {
	if request == nil || request.completed {
		return
	}
	request.completed = true
	request.err = err
	delete(m.pendingPings, requestID)
	request.done <- err
	close(request.done)
}
