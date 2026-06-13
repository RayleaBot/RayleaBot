package runtime

import (
	"context"
)

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

func (m *Manager) registerEventSession(ctx context.Context, handle *processHandle, requestID string, event Event) (*eventSession, *Error) {
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
