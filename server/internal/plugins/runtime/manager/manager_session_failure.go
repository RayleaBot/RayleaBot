package manager

import (
	"io"
	"time"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
)

func (m *Manager) failRuntime(handle *runtimeprocess.Handle, code, message string, err error) *Error {
	runtimeErr := errorf(code, message, err)

	m.mu.Lock()
	if m.proc != handle {
		m.mu.Unlock()
		return runtimeErr
	}
	m.markStoppedLocked(code, message, err)
	m.abortPendingLocked(runtimeErr)
	m.mu.Unlock()

	if handle != nil && handle.Cmd != nil && handle.Cmd.Process != nil {
		_ = handle.Cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}

	return runtimeErr
}

func (m *Manager) removeEventSession(handle *runtimeprocess.Handle, requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != handle {
		return
	}
	session := m.pendingEvents[requestID]
	if session == nil || session.completed {
		return
	}
	session.completed = true
	session.err = errorf(codePluginInternalError, "plugin runtime stopped before delivery completed", io.EOF)
	session.cancel()
	close(session.done)
	delete(m.pendingEvents, requestID)
}
