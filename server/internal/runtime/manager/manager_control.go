package manager

import (
	"context"
	"time"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
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
		"plugin runtime stopping",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopping),
	)

	writeErr := handle.WriteJSONLine(runtimeprotocol.ShutdownFrame{
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
			"plugin runtime stopped",
			"component", "runtime",
			"plugin_id", handle.Spec.PluginID,
			"runtime_state", string(StateStopped),
		)
		return nil
	case <-stopCtx.Done():
		return m.failRuntime(handle, codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
	}
}
