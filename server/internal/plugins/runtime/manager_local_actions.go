package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

const (
	// maxLocalActionHistory bounds the request IDs remembered per event to reject reuse.
	maxLocalActionHistory = 256
	// maxPendingLocalActions bounds outstanding local actions per plugin process;
	// further actions receive platform.rate_limited until earlier ones settle.
	maxPendingLocalActions = 256
)

type localActionRejection struct {
	parentRequestID string
	requestID       string
	code            string
	message         string
	details         map[string]any
}

func (m *Manager) routeLocalActionFrameLocked(handle *Handle, frame pluginwire.Frame) (*localActionRejection, *plugins.Error) {
	if frame.Propagation != "" {
		return nil, errorf(codePluginProtocolViolation, "nonterminal action cannot control propagation", nil)
	}
	action, parentRequestID, err := m.parseLocalActionFrameLocked(handle, frame)
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
	if action.Kind == "plugin.call" && m.snap.State == StateStopping {
		return &localActionRejection{parentRequestID: parentRequestID, requestID: frame.RequestID, code: codePluginServiceUnavailable, message: "service calls are unavailable while the caller is stopping"}, nil
	}
	if m.pendingLocalActions >= maxPendingLocalActions {
		return &localActionRejection{
			parentRequestID: parentRequestID,
			requestID:       frame.RequestID,
			code:            codePlatformRateLimited,
			message:         "plugin local action pending limit exceeded",
			details:         map[string]any{"limit": maxPendingLocalActions},
		}, nil
	}

	rememberLocalActionID(session, frame.RequestID, maxLocalActionHistory)
	session.pendingActionIDs[frame.RequestID] = struct{}{}
	session.pendingLocalAction++
	m.pendingLocalActions++

	actionCtx := session.ctx
	if action.Kind == "plugin.call" {
		var cancel context.CancelFunc
		if session.deadline.IsZero() {
			actionCtx, cancel = context.WithCancel(actionCtx)
		} else {
			actionCtx, cancel = context.WithDeadline(actionCtx, session.deadline)
		}
		if session.serviceCancels == nil {
			session.serviceCancels = make(map[string]context.CancelFunc)
		}
		session.serviceCancels[frame.RequestID] = cancel
	}
	go m.executeLocalAction(plugins.WithRuntimeDone(actionCtx, handle.Done()), handle, parentRequestID, frame.RequestID, *action, session.event)
	return nil, nil
}

// rememberLocalActionID evicts the oldest settled ID once the history is full.
// Pending IDs stay remembered, so the history exceeds the limit only while that
// many actions are still outstanding.
func rememberLocalActionID(session *eventSession, requestID string, limit int) {
	if limit < 1 {
		limit = 1
	}
	if len(session.localActionIDs) >= limit {
		for index, candidate := range session.localActionOrder {
			if _, pending := session.pendingActionIDs[candidate]; pending {
				continue
			}
			delete(session.localActionIDs, candidate)
			session.localActionOrder = append(session.localActionOrder[:index], session.localActionOrder[index+1:]...)
			break
		}
	}
	session.localActionIDs[requestID] = struct{}{}
	session.localActionOrder = append(session.localActionOrder, requestID)
}

func (m *Manager) parseLocalActionFrameLocked(handle *Handle, frame pluginwire.Frame) (*plugins.Action, string, *plugins.Error) {
	parentRequestID := strings.TrimSpace(frame.ParentRequestID)
	if parentRequestID == "" && frame.Action == "plugin.call" {
		return nil, "", errorf(codePluginProtocolViolation, "plugin.call requires parent_request_id", nil)
	}
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
	if action.Kind == "plugin.call" {
		var response any = pluginwire.ResultFrame{Type: "result", RequestID: requestID, Status: "success", Data: result}
		var failure *plugins.Error
		if errors.As(err, &failure) {
			response = localErrorFrame(requestID, failure.Code, failure.Message, failure.Details)
		}
		encoded, encodeErr := json.Marshal(response)
		if encodeErr != nil || len(encoded) > positiveInt(handle.Spec.IPCMessageMaxBytes, 8*1024*1024) {
			result = nil
			err = errorf(codePlatformValueTooLarge, "service response exceeds caller frame size", nil)
		}
	}
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
		if cancel := session.serviceCancels[requestID]; cancel != nil {
			cancel()
			delete(session.serviceCancels, requestID)
		}
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

// writeLocalRejectionLocked answers an action refused at admission; the caller
// holds protocolMu so the rejection precedes later terminal processing.
func (m *Manager) writeLocalRejectionLocked(handle *Handle, rejection localActionRejection) *plugins.Error {
	m.mu.RLock()
	session := m.pendingEvents[rejection.parentRequestID]
	active := m.proc == handle && session != nil && !session.completed
	m.mu.RUnlock()
	if !active {
		return nil
	}
	if err := handle.WriteJSONLine(localErrorFrame(rejection.requestID, rejection.code, rejection.message, rejection.details)); err != nil {
		return errorf(codePluginInternalError, "write local action rejection frame", err)
	}
	return nil
}
