package runtime

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"

	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (m *Manager) Stop(ctx context.Context) error {
	grace := m.ShutdownGrace()
	if err := m.acquireLifecycle(ctx); err != nil {
		return err
	}
	defer func() { <-m.lifecycleGate }()
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
		if err := handle.awaitProtocolDrain(ctx); err != nil {
			return m.failRuntime(handle, codePluginShutdownTimeout, "plugin output drain timed out", err)
		}
		m.mu.RLock()
		failure := handle.terminationError
		m.mu.RUnlock()
		if failure != nil {
			m.finishFailedProcess(handle, failure)
		} else {
			m.reconcileExitedProcess(handle, waitErr)
		}
		return nil
	}
	if failure := handle.terminationError; failure != nil {
		handle.failureRestart = false
		m.mu.Unlock()
		// Another operation already reported the failure and owns termination.
		// Stop waits for that cleanup without replacing its cause with a pipe error.
		cleanupCtx, cancel := context.WithTimeout(ctx, grace)
		defer cancel()
		select {
		case <-handle.Done():
			if err := handle.awaitProtocolDrain(cleanupCtx); err != nil {
				return m.failRuntime(handle, codePluginShutdownTimeout, "plugin output drain timed out", err)
			}
			m.finishFailedProcess(handle, failure)
			return nil
		case <-cleanupCtx.Done():
			return m.failRuntime(handle, codePluginShutdownTimeout, "plugin shutdown timed out", cleanupCtx.Err())
		}
	}
	handle.stopRequested = true
	m.snap.State = StateStopping
	m.retireServiceCallsLocked()
	m.cancelDetachedLocked()
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

	m.logger.Debug(
		"插件正在停止",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopping),
	)

	stopCtx, cancel := context.WithTimeout(ctx, grace)
	defer cancel()
	// Closing stdin releases a blocked event/action write and the shutdown
	// frame waiting behind it, so the deadline can reach process termination.
	stopPipe := context.AfterFunc(stopCtx, func() { _ = handle.Stdin.Close() })
	defer stopPipe()

	writeErr := handle.WriteJSONLine(pluginwire.ShutdownFrame{
		Type:      "shutdown",
		RequestID: m.deps.requestID(),
		Reason:    "stop",
	})
	_ = handle.Stdin.Close()
	if stopCtx.Err() != nil {
		return m.failRuntime(handle, codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
	}

	if writeErr != nil && !isIgnorableShutdownWriteError(writeErr) {
		return m.failRuntime(handle, codePluginInternalError, "write shutdown frame", writeErr)
	}

	select {
	case <-handle.Done():
		if err := handle.awaitProtocolDrain(stopCtx); err != nil {
			return m.failRuntime(handle, codePluginShutdownTimeout, "plugin output drain timed out", err)
		}
		m.mu.RLock()
		failure := handle.terminationError
		m.mu.RUnlock()
		if failure != nil {
			m.finishFailedProcess(handle, failure)
			return nil
		}
		waitErr, _ := handle.ExitResult()
		if waitErr != nil {
			m.markStopped(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
			return errorf(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
		}
		m.markStopped("", "", nil)
		m.logger.Info(
			"插件已停止",
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

func classifyProtocolReadError(handle *Handle, readErr error, exitMessage string, protocolMessage string) *plugins.Error {
	if errors.Is(readErr, errProtocolFrameTooLarge) {
		return errorf(codePluginProtocolViolation, "plugin IPC frame exceeds runtime.ipc_message_max_bytes", readErr)
	}
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
