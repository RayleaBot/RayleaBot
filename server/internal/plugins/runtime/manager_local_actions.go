package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func (m *Manager) routeLocalActionFrameLocked(handle *Handle, line []byte) *Error {
	frame, action, parentRequestID, err := m.parseLocalActionFrameLocked(handle, line)
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

	session.localActionIDs[frame.RequestID] = struct{}{}
	session.pendingLocalAction++

	go m.executeLocalAction(session.ctx, handle, parentRequestID, frame.RequestID, *action, session.event)
	return nil
}

func (m *Manager) parseLocalActionFrameLocked(handle *Handle, line []byte) (ActionFrame, *Action, string, *Error) {
	var frame ActionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return ActionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}

	parentRequestID := strings.TrimSpace(frame.ParentRequestID)
	if parentRequestID == "" {
		if handle.Spec.EffectiveConcurrency > 1 {
			return ActionFrame{}, nil, "", errorf(codePluginProtocolViolation, "concurrent plugin local actions must include parent_request_id", nil)
		}
		if len(m.pendingEvents) != 1 {
			return ActionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin local action parent_request_id is missing", nil)
		}
		for requestID := range m.pendingEvents {
			parentRequestID = requestID
		}
	}

	if m.eventExpiredLocked(parentRequestID) {
		return frame, nil, parentRequestID, nil
	}
	action, parseErr := ParseLocalAction(frame.Action, frame.Data)
	if parseErr != nil {
		return ActionFrame{}, nil, "", normalizeRuntimeError(parseErr, "parse local action frame")
	}
	return frame, action, parentRequestID, nil
}

func (m *Manager) executeLocalAction(ctx context.Context, handle *Handle, parentRequestID string, requestID string, action Action, parentEvent Event) {
	if m.opts.ExecuteLocalAction == nil {
		if err := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action executor is not available", nil); err != nil {
			_ = m.failRuntime(handle, err.Code, err.Message, err.Err)
		}
		return
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.Spec.PluginID, requestID, action, parentEvent)
	if err != nil {
		var runtimeErr *Error
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

func (m *Manager) writeLocalResult(handle *Handle, parentRequestID string, requestID string, data map[string]any) *Error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.Spec.PluginID,
		"request_id":       requestID,
		"status":           "success",
		"data":             data,
	}
	return m.writeLocalResponse(handle, parentRequestID, frame)
}

func (m *Manager) writeLocalError(handle *Handle, parentRequestID string, requestID string, code string, message string, details map[string]any) *Error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.Spec.PluginID,
		"request_id":       requestID,
		"code":             code,
		"message":          message,
	}
	if len(details) > 0 {
		frame["details"] = cloneDetails(details)
	}
	return m.writeLocalResponse(handle, parentRequestID, frame)
}

func (m *Manager) writeLocalResponse(handle *Handle, parentRequestID string, frame map[string]any) *Error {
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
	if session.pendingLocalAction > 0 {
		session.pendingLocalAction--
	}
	m.mu.Unlock()

	if err := handle.WriteJSONLine(frame); err != nil {
		return errorf(codePluginInternalError, "write local action response frame", err)
	}
	return nil
}
