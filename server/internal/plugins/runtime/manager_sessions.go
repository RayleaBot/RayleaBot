package runtime

import (
	"context"
	"io"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const expiredEventRetention = 5 * time.Minute

type eventSession struct {
	requestID string
	event     chatevent.Event
	// ctx does not follow the delivery context: DeliverEvent ends a foreground
	// event when its context ends, while a detached event outlives its delivery.
	ctx                context.Context
	cancel             context.CancelFunc
	startedAt          time.Time
	deadline           time.Time
	done               chan struct{}
	delivery           plugins.Delivery
	err                error
	localActionIDs     map[string]struct{}
	localActionOrder   []string
	pendingActionIDs   map[string]struct{}
	serviceCancels     map[string]context.CancelFunc
	pendingLocalAction int
	completed          bool
	// detachedCh is closed when event.detach completes the delivery; detached
	// holds the background state from then on.
	detachedCh chan struct{}
	detached   *detachedSession
}

func (m *Manager) registerEventSession(ctx context.Context, handle *Handle, requestID string, event chatevent.Event) (*eventSession, *plugins.Error) {
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	// The deadline is sent in the event frame, bounds the delivery timer and
	// keeps an outgoing service call from outliving its caller event.
	startedAt := time.Now()
	deadline := startedAt
	if handle != nil && handle.Spec.EventTimeout > 0 {
		deadline = startedAt.Add(handle.Spec.EventTimeout)
	}
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle || handle == nil {
		cancel()
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not running", plugins.ErrRuntimeNotRunning)
	}
	if m.snap.State == StateStopping {
		cancel()
		return nil, errorf(codePluginStopping, "plugin runtime is stopping", plugins.ErrRuntimeNotRunning)
	}
	if m.snap.State != StateRunning {
		cancel()
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not ready for event delivery", plugins.ErrRuntimeNotRunning)
	}
	if m.pendingEvents[requestID] != nil || m.eventExpiredLocked(requestID) {
		cancel()
		return nil, errorf(codePluginInternalError, "duplicate runtime request ID", nil)
	}
	if event.EventType == "plugin.request" {
		pending := 0
		for _, active := range m.pendingEvents {
			if active.event.EventType == "plugin.request" {
				pending++
			}
		}
		if pending >= maxPendingServiceCalls {
			cancel()
			return nil, errorWithDetails(codePlatformRateLimited, "service provider pending-call limit exceeded", map[string]any{"limit": maxPendingServiceCalls}, nil)
		}
	}

	session := &eventSession{
		requestID:        requestID,
		event:            event,
		ctx:              sessionCtx,
		cancel:           cancel,
		startedAt:        startedAt,
		deadline:         deadline,
		done:             make(chan struct{}),
		detachedCh:       make(chan struct{}),
		localActionIDs:   make(map[string]struct{}),
		pendingActionIDs: make(map[string]struct{}),
	}
	m.pendingEvents[requestID] = session
	return session, nil
}

func (m *Manager) completeEventLocked(session *eventSession, delivery plugins.Delivery, err error) {
	if session == nil || session.completed || m.pendingEvents[session.requestID] != session {
		return
	}
	m.closeSessionLocked(session, delivery, err)
}

// closeSessionLocked is the single exit of an event session; a detached event
// also reports its end to the dispatcher that released it.
func (m *Manager) closeSessionLocked(session *eventSession, delivery plugins.Delivery, err error) {
	session.completed = true
	session.delivery = delivery
	session.err = err
	m.releaseSessionActionsLocked(session)
	delete(m.pendingEvents, session.requestID)
	session.cancel()
	close(session.done)
	if session.detached != nil {
		m.endDetachedLocked(session, err)
	}
}

// settledDelivery reads the delivery result once the session was detached or
// completed. After event.detach the caller receives the detach result even
// when the terminal has already arrived.
func (m *Manager) settledDelivery(session *eventSession) (plugins.Delivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if session.detached != nil {
		return session.detached.delivery, nil
	}
	return session.delivery, session.err
}

func (m *Manager) releaseSessionActionsLocked(session *eventSession) {
	if session != nil {
		for _, cancel := range session.serviceCancels {
			cancel()
		}
		clear(session.serviceCancels)
	}
	if session == nil || session.pendingLocalAction <= 0 {
		return
	}
	m.pendingLocalActions -= session.pendingLocalAction
	if m.pendingLocalActions < 0 {
		m.pendingLocalActions = 0
	}
	session.pendingLocalAction = 0
	clear(session.pendingActionIDs)
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

func (m *Manager) failRuntime(handle *Handle, code, message string, err error) *plugins.Error {
	runtimeErr := errorf(code, message, err)

	m.mu.Lock()
	if m.proc != handle || handle == nil {
		m.mu.Unlock()
		return runtimeErr
	}
	runtimeErr.MarkFailureReported()
	observe := !handle.terminationObserved
	if observe {
		handle.failureRestart = m.snap.State == StateRunning && m.opts.OnCrash != nil
		if handle.failureRestart {
			m.snap.CrashCount++
		}
		handle.terminationError = runtimeErr
		m.snap.LastErrorCode = code
		m.snap.LastErrorMessage = runtimeErr.Error()
	}
	m.snap.State = StateStopping
	handle.terminationObserved = true
	m.abortPendingLocked(runtimeErr)
	m.mu.Unlock()
	if observe {
		m.logger.Warn("插件发生错误，正在回收进程", "component", "runtime", "plugin_id", handle.Spec.PluginID, "error_code", code, "reason", runtimeErr.Error())
	}

	if handle.Stdin != nil {
		_ = handle.Stdin.Close()
	}
	handle.killTree()
	handle.closeStdout()
	select {
	case <-handle.Done():
		m.finishFailedProcess(handle, runtimeErr)
	case <-time.After(config.PluginKillWait):
		if observe {
			go func() { <-handle.Done(); m.finishFailedProcess(handle, runtimeErr) }()
		}
	}

	return runtimeErr
}

func (m *Manager) finishFailedProcess(handle *Handle, runtimeErr *plugins.Error) {
	m.mu.Lock()
	var onCrash CrashCallback
	var pluginID string
	var count int
	if m.proc == handle {
		if handle.terminationError != nil {
			runtimeErr = handle.terminationError
		}
		m.markStoppedLocked(runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		if handle.failureRestart {
			m.snap.State = StateCrashed
			onCrash, pluginID, count = m.opts.OnCrash, m.snap.PluginID, m.snap.CrashCount
		}
	}
	m.mu.Unlock()
	if onCrash != nil {
		go onCrash(pluginID, count, runtimeErr.Code)
	}
}

func (m *Manager) timeoutEvent(handle *Handle, session *eventSession, code, message string, err error) (plugins.Delivery, error) {
	runtimeErr := errorf(code, message, err)
	if session == nil {
		return plugins.Delivery{}, runtimeErr
	}

	delivery := plugins.Delivery{
		RequestID:    session.requestID,
		ErrorCode:    runtimeErr.Code,
		ErrorMessage: runtimeErr.Message,
		ErrorDetails: cloneDetails(runtimeErr.Details),
	}
	// Settle the caller's outstanding service calls after releasing the manager
	// lock. A timed-out plugin.request is not announced to its provider, which
	// stops at deadline_at_ms; its late terminal frame matches an expired event.
	var canceledCalls []string
	defer func() {
		for _, requestID := range canceledCalls {
			_ = handle.WriteJSONLine(localErrorFrame(requestID, code, message, nil))
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()
	if session.detached != nil {
		// event.detach won the race against the foreground deadline.
		return session.detached.delivery, nil
	}
	if session.completed {
		if session.err == nil {
			return session.delivery, nil
		}
		if runtimeSessionErr, ok := session.err.(*plugins.Error); ok {
			return session.delivery, runtimeSessionErr
		}
		return session.delivery, errorf(codePluginInternalError, "plugin event delivery failed", session.err)
	}
	if m.proc != handle || m.pendingEvents[session.requestID] != session {
		return delivery, runtimeErr
	}
	for requestID := range session.serviceCancels {
		canceledCalls = append(canceledCalls, requestID)
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
	m.closeSessionLocked(session, plugins.Delivery{}, errorf(codePluginInternalError, "plugin runtime stopped before delivery completed", io.EOF))
}
