package runtime

import (
	"context"
	"io"
	"time"
)

const expiredEventRetention = 5 * time.Minute

type eventSession struct {
	requestID          string
	event              Event
	ctx                context.Context
	cancel             context.CancelFunc
	done               chan struct{}
	delivery           Delivery
	err                error
	localActionIDs     map[string]struct{}
	pendingLocalAction int
	completed          bool
}

type pingRequest struct {
	done      chan error
	err       error
	completed bool
}

func (m *Manager) registerEventSession(ctx context.Context, handle *Handle, requestID string, event Event) (*eventSession, *Error) {
	sessionCtx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle || handle == nil {
		cancel()
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if m.snap.State == StateStopping {
		cancel()
		return nil, errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if m.snap.State != StateRunning {
		cancel()
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not ready for event delivery", nil)
	}

	session := &eventSession{
		requestID:      requestID,
		event:          event,
		ctx:            sessionCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
		localActionIDs: make(map[string]struct{}),
	}
	m.pendingEvents[requestID] = session
	return session, nil
}

func (m *Manager) completeEventLocked(session *eventSession, delivery Delivery, err error) {
	if session == nil || session.completed {
		return
	}
	session.completed = true
	session.delivery = delivery
	session.err = err
	delete(m.pendingEvents, session.requestID)
	session.cancel()
	close(session.done)
}

func (m *Manager) markEventExpiredLocked(requestID string) {
	if requestID == "" {
		return
	}
	now := m.deps.now()
	m.pruneExpiredEventsLocked(now)
	if m.expiredEvents == nil {
		m.expiredEvents = make(map[string]time.Time)
	}
	m.expiredEvents[requestID] = now.Add(expiredEventRetention)
}

func (m *Manager) eventExpiredLocked(requestID string) bool {
	if requestID == "" {
		return false
	}
	expiresAt, ok := m.expiredEvents[requestID]
	if !ok {
		return false
	}
	now := m.deps.now()
	if !expiresAt.IsZero() && now.After(expiresAt) {
		delete(m.expiredEvents, requestID)
		return false
	}
	return true
}

func (m *Manager) pruneExpiredEventsLocked(now time.Time) {
	for requestID, expiresAt := range m.expiredEvents {
		if !expiresAt.IsZero() && now.After(expiresAt) {
			delete(m.expiredEvents, requestID)
		}
	}
}

func (m *Manager) failRuntime(handle *Handle, code, message string, err error) *Error {
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

func (m *Manager) timeoutEvent(handle *Handle, session *eventSession, code, message string, err error) (Delivery, *Error) {
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

func (m *Manager) removeEventSession(handle *Handle, requestID string) {
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
