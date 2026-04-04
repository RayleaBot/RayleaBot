package runtime

import (
	"context"
	"encoding/json"
	"errors"
)

func (m *Manager) handleLocalActionFrame(ctx context.Context, handle *processHandle, envelope frameEnvelope, seenLocalRequestIDs map[string]struct{}, line []byte) error {
	if envelope.Type != "action" {
		return errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during local action handling", nil)
	}
	if _, exists := seenLocalRequestIDs[envelope.RequestID]; exists {
		return errorf(codePluginProtocolViolation, "plugin reused a local action request_id within one event delivery", nil)
	}

	var frame actionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}

	var action *Action
	var err error
	switch frame.Action {
	case "logger.write":
		action, err = parseLoggerWriteAction(frame.Data)
	case "storage.kv":
		action, err = parseStorageKVAction(frame.Data)
	case "config.read":
		action, err = parseConfigReadAction(frame.Data)
	case "config.write":
		action, err = parseConfigWriteAction(frame.Data)
	case "storage.file":
		action, err = parseStorageFileAction(frame.Data)
	case "http.request":
		action, err = parseHTTPRequestAction(frame.Data)
	case "scheduler.create":
		action, err = parseSchedulerCreateAction(frame.Data)
	case "event.expose_webhook":
		action, err = parseEventExposeWebhookAction(frame.Data)
	case "render.image":
		action, err = parseRenderImageAction(frame.Data)
	case "message.send", "message.reply":
		return errorf(codePluginProtocolViolation, "terminal message actions must use the current event request_id", nil)
	default:
		return errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
	}
	if err != nil {
		return err
	}

	seenLocalRequestIDs[envelope.RequestID] = struct{}{}
	return m.executeLocalAction(ctx, handle, envelope.RequestID, *action)
}

func (m *Manager) executeLocalAction(ctx context.Context, handle *processHandle, requestID string, action Action) error {
	if m.opts.ExecuteLocalAction == nil {
		return errorf(codePluginInternalError, "plugin local action executor is not available", nil)
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.spec.PluginID, requestID, action)
	if err != nil {
		var runtimeErr *Error
		if errors.As(err, &runtimeErr) {
			return m.writeLocalError(handle, requestID, runtimeErr.Code, runtimeErr.Message)
		}
		return m.writeLocalError(handle, requestID, codePluginInternalError, "plugin local action failed")
	}

	if result == nil {
		result = map[string]any{}
	}
	return m.writeLocalResult(handle, requestID, result)
}

func (m *Manager) writeLocalResult(handle *processHandle, requestID string, data map[string]any) error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"status":           "success",
		"data":             data,
	}
	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return errorf(codePluginInternalError, "write local action result frame", err)
	}
	return nil
}

func (m *Manager) writeLocalError(handle *processHandle, requestID string, code string, message string) error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"code":             code,
		"message":          message,
	}
	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return errorf(codePluginInternalError, "write local action error frame", err)
	}
	return nil
}
