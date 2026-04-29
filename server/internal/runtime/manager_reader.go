package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
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

func (m *Manager) readRuntimeFrames(handle *processHandle) {
	for {
		line, err := handle.stdout.ReadBytes('\n')
		if err != nil {
			runtimeErr := classifyProtocolReadError(handle, err, "plugin exited during runtime delivery", "read plugin runtime response")
			if errorsAreExitLike(handle, err) {
				m.signalPendingRequests(handle, runtimeErr)
				return
			}
			m.failRuntime(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return
		}

		m.protocolMu.Lock()
		runtimeErr := m.routeRuntimeFrame(handle, line)
		m.protocolMu.Unlock()
		if runtimeErr != nil {
			m.failRuntime(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return
		}
	}
}

func errorsAreExitLike(handle *processHandle, err error) bool {
	if isProcessPipeClosedError(err) {
		return true
	}
	if handle == nil {
		return false
	}
	_, exited := handle.exitResult()
	return exited
}

func (m *Manager) routeRuntimeFrame(handle *processHandle, line []byte) *Error {
	envelope, err := parseEventEnvelope(line, handle.spec.PluginID)
	if err != nil {
		return normalizeRuntimeError(err, "parse runtime frame envelope")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle {
		return nil
	}

	if ping := m.pendingPings[envelope.RequestID]; ping != nil {
		if envelope.Type != "pong" {
			return errorf(codePluginProtocolViolation, "plugin returned unexpected frame type in response to ping", nil)
		}
		m.completePingLocked(envelope.RequestID, ping, nil)
		return nil
	}

	if session := m.pendingEvents[envelope.RequestID]; session != nil {
		return m.routeTerminalFrameLocked(session, envelope, line)
	}

	if envelope.Type == "action" {
		return m.routeLocalActionFrameLocked(handle, line)
	}

	return errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during runtime delivery", nil)
}

func (m *Manager) routeTerminalFrameLocked(session *eventSession, envelope frameEnvelope, line []byte) *Error {
	if session.pendingLocalAction > 0 {
		return errorf(codePluginProtocolViolation, "plugin returned a terminal frame before all local actions completed", nil)
	}

	delivery, done, err := decodeTerminalDelivery(session.requestID, line, envelope.Type)
	if !done {
		return errorf(codePluginProtocolViolation, "plugin returned an unexpected non-terminal frame for the active event", nil)
	}
	if err != nil {
		var runtimeErr *Error
		if ok := asRuntimeError(err, &runtimeErr); ok {
			m.completeEventLocked(session, delivery, runtimeErr)
			return nil
		}
		m.completeEventLocked(session, delivery, errorf(codePluginInternalError, "terminal frame returned unexpected error", err))
		return nil
	}

	m.completeEventLocked(session, delivery, nil)
	return nil
}

func asRuntimeError(err error, target **Error) bool {
	if err == nil {
		return false
	}
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		return false
	}
	*target = runtimeErr
	return true
}

func normalizeRuntimeError(err error, message string) *Error {
	if err == nil {
		return nil
	}
	var runtimeErr *Error
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	return errorf(codePluginInternalError, message, err)
}

func (m *Manager) routeLocalActionFrameLocked(handle *processHandle, line []byte) *Error {
	frame, action, parentRequestID, err := m.parseLocalActionFrameLocked(handle, line)
	if err != nil {
		return err
	}

	session := m.pendingEvents[parentRequestID]
	if session == nil {
		return errorf(codePluginProtocolViolation, "plugin local action parent_request_id does not match an active event", nil)
	}
	if frame.RequestID == session.requestID {
		return errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
	}
	if _, exists := session.localActionIDs[frame.RequestID]; exists {
		return errorf(codePluginProtocolViolation, "plugin reused a local action request_id within one event delivery", nil)
	}

	session.localActionIDs[frame.RequestID] = struct{}{}
	session.pendingLocalAction++

	go m.executeLocalAction(session.ctx, handle, parentRequestID, frame.RequestID, *action, session.event)
	return nil
}

func (m *Manager) parseLocalActionFrameLocked(handle *processHandle, line []byte) (actionFrame, *Action, string, *Error) {
	var frame actionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}

	parentRequestID := strings.TrimSpace(frame.ParentRequestID)
	if parentRequestID == "" {
		if handle.spec.EffectiveConcurrency > 1 {
			return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "concurrent plugin local actions must include parent_request_id", nil)
		}
		if len(m.pendingEvents) != 1 {
			return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin local action parent_request_id is missing", nil)
		}
		for requestID := range m.pendingEvents {
			parentRequestID = requestID
		}
	}

	var action *Action
	var parseErr error
	switch frame.Action {
	case "logger.write":
		action, parseErr = parseLoggerWriteAction(frame.Data)
	case "storage.kv":
		action, parseErr = parseStorageKVAction(frame.Data)
	case "config.read":
		action, parseErr = parseConfigReadAction(frame.Data)
	case "plugin.list":
		action, parseErr = parsePluginListAction(frame.Data)
	case "config.write":
		action, parseErr = parseConfigWriteAction(frame.Data)
	case "governance.blacklist.read":
		action, parseErr = parseGovernanceBlacklistReadAction(frame.Data)
	case "governance.blacklist.write":
		action, parseErr = parseGovernanceBlacklistWriteAction(frame.Data)
	case "governance.whitelist.read":
		action, parseErr = parseGovernanceWhitelistReadAction(frame.Data)
	case "governance.whitelist.write":
		action, parseErr = parseGovernanceWhitelistWriteAction(frame.Data)
	case "governance.command_policy.read":
		action, parseErr = parseGovernanceCommandPolicyReadAction(frame.Data)
	case "storage.file":
		action, parseErr = parseStorageFileAction(frame.Data)
	case "http.request":
		action, parseErr = parseHTTPRequestAction(frame.Data)
	case "scheduler.create":
		action, parseErr = parseSchedulerCreateAction(frame.Data)
	case "event.expose_webhook":
		action, parseErr = parseEventExposeWebhookAction(frame.Data)
	case "render.image":
		action, parseErr = parseRenderImageAction(frame.Data)
	case "message.send", "message.reply":
		return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "terminal message actions must use the current event request_id", nil)
	default:
		switch {
		case isOneBotFamilyAction(frame.Action), isProviderExtensionAction(frame.Action):
			action, parseErr = parseOneBotFamilyAction(frame.Action, frame.Data)
		default:
			return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
		}
	}
	if parseErr != nil {
		return actionFrame{}, nil, "", normalizeRuntimeError(parseErr, "parse local action frame")
	}
	return frame, action, parentRequestID, nil
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

func (m *Manager) registerPingRequest(handle *processHandle, requestID string) (*pingRequest, *Error) {
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

func (m *Manager) signalPendingRequests(handle *processHandle, runtimeErr *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != handle {
		return
	}
	m.abortPendingLocked(runtimeErr)
}

func (m *Manager) failRuntime(handle *processHandle, code, message string, err error) *Error {
	runtimeErr := errorf(code, message, err)

	m.mu.Lock()
	if m.proc != handle {
		m.mu.Unlock()
		return runtimeErr
	}
	m.markStoppedLocked(code, message, err)
	m.abortPendingLocked(runtimeErr)
	m.mu.Unlock()

	if handle != nil && handle.cmd != nil && handle.cmd.Process != nil {
		_ = handle.cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
	}

	return runtimeErr
}

func (m *Manager) removeEventSession(handle *processHandle, requestID string) {
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
