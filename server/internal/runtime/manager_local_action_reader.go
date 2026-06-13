package runtime

import (
	"encoding/json"
	"strings"
)

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
	case "secret.read":
		action, parseErr = parseSecretReadAction(frame.Data)
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
