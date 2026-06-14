package manager

import runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"

func (m *Manager) abortPendingLocked(runtimeErr *Error) {
	for requestID, session := range m.pendingEvents {
		if session.completed {
			delete(m.pendingEvents, requestID)
			continue
		}
		session.completed = true
		session.err = runtimeErr
		session.cancel()
		close(session.done)
		delete(m.pendingEvents, requestID)
	}

	for requestID, ping := range m.pendingPings {
		if ping.completed {
			delete(m.pendingPings, requestID)
			continue
		}
		ping.completed = true
		ping.err = runtimeErr
		ping.done <- runtimeErr
		close(ping.done)
		delete(m.pendingPings, requestID)
	}
}

func (m *Manager) signalPendingRequests(handle *runtimeprocess.Handle, runtimeErr *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != handle {
		return
	}
	m.abortPendingLocked(runtimeErr)
}
