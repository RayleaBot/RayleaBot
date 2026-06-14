package manager

import (
	"context"
	"errors"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/runtime/process"
)

func (m *Manager) executeLocalAction(ctx context.Context, handle *runtimeprocess.Handle, parentRequestID string, requestID string, action Action, parentEvent Event) {
	if m.opts.ExecuteLocalAction == nil {
		if err := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action executor is not available", nil); err != nil {
			m.failRuntime(handle, err.Code, err.Message, err.Err)
		}
		return
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.Spec.PluginID, requestID, action, parentEvent)
	if err != nil {
		var runtimeErr *Error
		if errors.As(err, &runtimeErr) {
			if writeErr := m.writeLocalError(handle, parentRequestID, requestID, runtimeErr.Code, runtimeErr.Message, runtimeErr.Details); writeErr != nil {
				m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
			}
			return
		}
		if writeErr := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action failed", nil); writeErr != nil {
			m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
		}
		return
	}

	if result == nil {
		result = map[string]any{}
	}
	if err := m.writeLocalResult(handle, parentRequestID, requestID, result); err != nil {
		m.failRuntime(handle, err.Code, err.Message, err.Err)
	}
}

func (m *Manager) writeLocalResult(handle *runtimeprocess.Handle, parentRequestID string, requestID string, data map[string]any) *Error {
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

func (m *Manager) writeLocalError(handle *runtimeprocess.Handle, parentRequestID string, requestID string, code string, message string, details map[string]any) *Error {
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

func (m *Manager) writeLocalResponse(handle *runtimeprocess.Handle, parentRequestID string, frame map[string]any) *Error {
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
