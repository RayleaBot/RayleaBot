package manager

import (
	"errors"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (m *Manager) readRuntimeFrames(handle *runtimeprocess.Handle) {
	for {
		line, err := handle.Stdout.ReadBytes('\n')
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

func errorsAreExitLike(handle *runtimeprocess.Handle, err error) bool {
	if isProcessPipeClosedError(err) {
		return true
	}
	if handle == nil {
		return false
	}
	_, exited := handle.ExitResult()
	return exited
}

func (m *Manager) routeRuntimeFrame(handle *runtimeprocess.Handle, line []byte) *Error {
	envelope, err := parseEventEnvelope(line, handle.Spec.PluginID)
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

func (m *Manager) routeTerminalFrameLocked(session *eventSession, envelope runtimeprotocol.FrameEnvelope, line []byte) *Error {
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
	var actionErr *runtimeaction.Error
	if errors.As(err, &actionErr) {
		return errorf(actionErr.Code, actionErr.Message, actionErr.Err)
	}
	return errorf(codePluginInternalError, message, err)
}
