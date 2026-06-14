package manager

import (
	"encoding/json"
	"strings"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/runtime/process"
)

func (m *Manager) routeLocalActionFrameLocked(handle *runtimeprocess.Handle, line []byte) *Error {
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

func (m *Manager) parseLocalActionFrameLocked(handle *runtimeprocess.Handle, line []byte) (actionFrame, *Action, string, *Error) {
	var frame actionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}

	parentRequestID := strings.TrimSpace(frame.ParentRequestID)
	if parentRequestID == "" {
		if handle.Spec.EffectiveConcurrency > 1 {
			return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "concurrent plugin local actions must include parent_request_id", nil)
		}
		if len(m.pendingEvents) != 1 {
			return actionFrame{}, nil, "", errorf(codePluginProtocolViolation, "plugin local action parent_request_id is missing", nil)
		}
		for requestID := range m.pendingEvents {
			parentRequestID = requestID
		}
	}

	action, parseErr := runtimeaction.ParseLocalAction(frame.Action, frame.Data)
	if parseErr != nil {
		return actionFrame{}, nil, "", normalizeRuntimeError(parseErr, "parse local action frame")
	}
	return frame, action, parentRequestID, nil
}
