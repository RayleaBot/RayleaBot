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

func (m *Manager) timeoutEvent(handle *runtimeprocess.Handle, session *eventSession, code, message string, err error) (Delivery, *Error) {
	runtimeErr := errorf(code, message, err)
	if session == nil {
		return Delivery{}, runtimeErr
	}

	delivery := Delivery{
		RequestID:    session.requestID,
		ErrorCode:    runtimeErr.Code,
		ErrorMessage: runtimeErr.Message,
		ErrorDetails: cloneDetails(runtimeErr.Details),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if session.completed {
		if session.err == nil {
			return session.delivery, nil
		}
		if runtimeSessionErr, ok := session.err.(*Error); ok {
			return session.delivery, runtimeSessionErr
		}
		return session.delivery, errorf(codePluginInternalError, "plugin event delivery failed", session.err)
	}
	if m.proc != handle {
		return delivery, runtimeErr
	}
	m.completeEventLocked(session, delivery, runtimeErr)
	m.markEventExpiredLocked(session.requestID)
	return delivery, runtimeErr
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
