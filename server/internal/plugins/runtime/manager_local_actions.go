package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

// maxLocalActionHistory bounds the request IDs remembered per event to reject reuse.
const maxLocalActionHistory = 256

func (m *Manager) routeLocalActionFrameLocked(handle *Handle, frame pluginwire.Frame) *plugins.Error {
	if frame.Propagation != "" {
		return errorf(codePluginProtocolViolation, "nonterminal action cannot control propagation", nil)
	}
	action, parentRequestID, err := m.parseLocalActionFrameLocked(handle, frame)
	if err != nil {
		return err
	}
	if action == nil && m.eventExpiredLocked(parentRequestID) {
		return nil
	}

	session := m.pendingEvents[parentRequestID]
	if session == nil {
		if m.eventExpiredLocked(parentRequestID) {
			return nil
		}
		return errorf(codePluginProtocolViolation, "plugin local action parent_request_id does not match an active event", nil)
	}
	if frame.RequestID == session.requestID {
		return errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
	}
	if _, exists := session.localActionIDs[frame.RequestID]; exists {
		return errorf(codePluginProtocolViolation, "plugin reused a local action request_id within one event delivery", nil)
	}

	rememberLocalActionID(session, frame.RequestID, maxLocalActionHistory)
	session.pendingActionIDs[frame.RequestID] = struct{}{}
	session.pendingLocalAction++

	go m.executeLocalAction(plugins.WithRuntimeDone(session.ctx, handle.Done()), handle, parentRequestID, frame.RequestID, *action, session.event)
	return nil
}

func rememberLocalActionID(session *eventSession, requestID string, limit int) {
	if limit < 1 {
		limit = 1
	}
	for len(session.localActionIDs) >= limit && len(session.localActionOrder) > 0 {
		candidate := session.localActionOrder[0]
		session.localActionOrder = session.localActionOrder[1:]
		if _, pending := session.pendingActionIDs[candidate]; pending {
			session.localActionOrder = append(session.localActionOrder, candidate)
			continue
		}
		delete(session.localActionIDs, candidate)
		break
	}
	session.localActionIDs[requestID] = struct{}{}
	session.localActionOrder = append(session.localActionOrder, requestID)
}

func (m *Manager) parseLocalActionFrameLocked(handle *Handle, frame pluginwire.Frame) (*plugins.Action, string, *plugins.Error) {
	parentRequestID := strings.TrimSpace(frame.ParentRequestID)
	if parentRequestID == "" {
		if handle.Spec.EffectiveConcurrency > 1 {
			return nil, "", errorf(codePluginProtocolViolation, "concurrent plugin local actions must include parent_request_id", nil)
		}
		if len(m.pendingEvents) != 1 {
			return nil, "", errorf(codePluginProtocolViolation, "plugin local action parent_request_id is missing", nil)
		}
		for requestID := range m.pendingEvents {
			parentRequestID = requestID
		}
	}

	if m.eventExpiredLocked(parentRequestID) {
		return nil, parentRequestID, nil
	}
	action, parseErr := ParseLocalAction(frame.Action, frame.Data)
	if parseErr != nil {
		return nil, "", normalizeRuntimeError(parseErr, "parse local action frame")
	}
	return action, parentRequestID, nil
}

func (m *Manager) executeLocalAction(ctx context.Context, handle *Handle, parentRequestID string, requestID string, action plugins.Action, parentEvent chatevent.Event) {
	ctx = plugins.WithParentRequestID(ctx, parentRequestID)
	if m.opts.ExecuteLocalAction == nil {
		if err := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action executor is not available", nil); err != nil {
			_ = m.failRuntime(handle, err.Code, err.Message, err.Err)
		}
		return
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.Spec.PluginID, requestID, action, parentEvent)
	if err != nil {
		var runtimeErr *plugins.Error
		if errors.As(err, &runtimeErr) {
			if writeErr := m.writeLocalError(handle, parentRequestID, requestID, runtimeErr.Code, runtimeErr.Message, runtimeErr.Details); writeErr != nil {
				_ = m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
			}
			return
		}
		if writeErr := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action failed", nil); writeErr != nil {
			_ = m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
		}
		return
	}

	if result == nil {
		result = map[string]any{}
	}
	if err := m.writeLocalResult(handle, parentRequestID, requestID, result); err != nil {
		_ = m.failRuntime(handle, err.Code, err.Message, err.Err)
	}
}

func (m *Manager) writeLocalResult(handle *Handle, parentRequestID string, requestID string, data map[string]any) *plugins.Error {
	frame := pluginwire.ResultFrame{Type: "result", RequestID: requestID, Status: "success", Data: data}
	return m.writeLocalResponse(handle, parentRequestID, requestID, frame)
}

func (m *Manager) writeLocalError(handle *Handle, parentRequestID string, requestID string, code string, message string, details map[string]any) *plugins.Error {
	return m.writeLocalResponse(handle, parentRequestID, requestID, localErrorFrame(requestID, code, message, details))
}

func localErrorFrame(requestID, code, message string, details map[string]any) pluginwire.ErrorFrame {
	frame := pluginwire.ErrorFrame{Type: "error", RequestID: requestID, Code: code, Message: message}
	if len(details) > 0 {
		frame.Details = cloneDetails(details)
	}
	return frame
}

// Response writes and inbound terminal frames share this ordering boundary.
// The protocol lock keeps terminal processing behind the pending response write.
func (m *Manager) writeLocalResponse(handle *Handle, parentRequestID string, requestID string, frame any) *plugins.Error {
	m.protocolMu.Lock()
	defer m.protocolMu.Unlock()

	m.mu.Lock()
	if m.proc != handle {
		m.mu.Unlock()
		return nil
	}
	session := m.pendingEvents[parentRequestID]
	if session == nil || session.completed {
		m.mu.Unlock()
		return nil
	}
	if _, pending := session.pendingActionIDs[requestID]; pending {
		delete(session.pendingActionIDs, requestID)
		session.pendingLocalAction--
	}
	m.mu.Unlock()

	if err := handle.WriteJSONLine(frame); err != nil {
		return errorf(codePluginInternalError, "write local action response frame", err)
	}
	return nil
}
