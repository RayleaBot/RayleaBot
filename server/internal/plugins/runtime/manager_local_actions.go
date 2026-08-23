package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type localActionRejection struct {
	parentRequestID string
	requestID       string
	code            string
	message         string
	details         map[string]any
}

func (m *Manager) routeLocalActionFrameLocked(handle *Handle, line []byte) (*localActionRejection, *Error) {
	frame, action, parentRequestID, err := m.parseLocalActionFrameLocked(handle, line)
	if err != nil {
		return nil, err
	}
	if action == nil && m.eventExpiredLocked(parentRequestID) {
		return nil, nil
	}

	session := m.pendingEvents[parentRequestID]
	if session == nil {
		if m.eventExpiredLocked(parentRequestID) {
			return nil, nil
		}
		return nil, errorf(codePluginProtocolViolation, "plugin local action parent_request_id does not match an active event", nil)
	}
	if frame.RequestID == session.requestID {
		return nil, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
	}
	if _, exists := session.localActionIDs[frame.RequestID]; exists {
		return nil, errorf(codePluginProtocolViolation, "plugin reused a local action request_id within one event delivery", nil)
	}
	if !m.allowActionBurstLocked(handle) {
		return &localActionRejection{
			parentRequestID: parentRequestID,
			requestID:       frame.RequestID,
			code:            codePlatformRateLimited,
			message:         "plugin IPC action burst limit exceeded",
			details: map[string]any{
				"limit_scope": "runtime.ipc_action_burst_limit",
				"limit":       handle.Spec.IPCActionBurstCount,
			},
		}, nil
	}
	if m.pendingLocalActions >= handle.Spec.IPCPendingActionsMax {
		return &localActionRejection{
			parentRequestID: parentRequestID,
			requestID:       frame.RequestID,
			code:            codePlatformRateLimited,
			message:         "plugin IPC pending action limit exceeded",
			details: map[string]any{
				"limit_scope": "runtime.ipc_pending_actions_max",
				"limit":       handle.Spec.IPCPendingActionsMax,
			},
		}, nil
	}

	rememberLocalActionID(session, frame.RequestID, handle.Spec.IPCPendingActionsMax)
	session.pendingActionIDs[frame.RequestID] = struct{}{}
	session.pendingLocalAction++
	m.pendingLocalActions++

	go m.executeLocalAction(session.ctx, handle, parentRequestID, frame.RequestID, *action, session.event)
	return nil, nil
}

func (m *Manager) allowActionBurstLocked(handle *Handle) bool {
	limit := handle.Spec.IPCActionBurstCount
	window := handle.Spec.IPCActionBurstWindow
	if limit <= 0 || window <= 0 {
		return true
	}
	now := m.deps.now()
	if m.actionBurstStarted.IsZero() || !now.Before(m.actionBurstStarted.Add(window)) {
		m.actionBurstStarted = now
		m.actionBurstCount = 0
	}
	if m.actionBurstCount >= limit {
		return false
	}
	m.actionBurstCount++
	return true
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
	requestID, _ := frame["request_id"].(string)
	if _, pending := session.pendingActionIDs[requestID]; pending {
		delete(session.pendingActionIDs, requestID)
		session.pendingLocalAction--
		if m.pendingLocalActions > 0 {
			m.pendingLocalActions--
		}
	}
	m.mu.Unlock()

	if err := handle.WriteJSONLine(frame); err != nil {
		return errorf(codePluginInternalError, "write local action response frame", err)
	}
	return nil
}

func (m *Manager) writeLocalRejectionLocked(handle *Handle, rejection localActionRejection) *Error {
	m.mu.RLock()
	if m.proc != handle {
		m.mu.RUnlock()
		return nil
	}
	session := m.pendingEvents[rejection.parentRequestID]
	active := session != nil && !session.completed
	m.mu.RUnlock()
	if !active {
		return nil
	}
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.Spec.PluginID,
		"request_id":       rejection.requestID,
		"code":             rejection.code,
		"message":          rejection.message,
	}
	if len(rejection.details) > 0 {
		frame["details"] = cloneDetails(rejection.details)
	}
	if err := handle.WriteJSONLine(frame); err != nil {
		return errorf(codePluginInternalError, "write local action rejection frame", err)
	}
	return nil
}
