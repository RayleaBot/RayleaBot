package runtime

import "encoding/json"

func parseEventEnvelope(line []byte, pluginID string) (frameEnvelope, error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return frameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return frameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return frameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" {
		return frameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}
	return envelope, nil
}

func decodeTerminalDelivery(eventRequestID string, line []byte, frameType string) (Delivery, bool, error) {
	switch frameType {
	case "action":
		return decodeTerminalAction(eventRequestID, line)
	case "result":
		return decodeTerminalResult(eventRequestID, line)
	case "error":
		return decodeTerminalError(eventRequestID, line)
	default:
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during event delivery", nil)
	}
}

func decodeTerminalAction(eventRequestID string, line []byte) (Delivery, bool, error) {
	var frame actionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}
	switch frame.Action {
	case "message.send":
		action, err := parseMessageSendAction(frame.Data)
		if err != nil {
			return Delivery{}, false, err
		}
		return Delivery{RequestID: eventRequestID, Action: action}, true, nil
	case "message.reply":
		action, err := parseMessageReplyAction(frame.Data)
		if err != nil {
			return Delivery{}, false, err
		}
		return Delivery{RequestID: eventRequestID, Action: action}, true, nil
	case "logger.write", "storage.kv", "config.read", "plugin.list", "secret.read", "config.write", "storage.file", "http.request", "scheduler.create", "event.expose_webhook", "render.image":
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
	default:
		if isOneBotFamilyAction(frame.Action) || isProviderExtensionAction(frame.Action) {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
		}
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
	}
}

func decodeTerminalResult(eventRequestID string, line []byte) (Delivery, bool, error) {
	var frame resultFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed result frame", err)
	}
	if frame.Status != "success" {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin result frame must use status=success", nil)
	}
	if frame.Data == nil {
		frame.Data = map[string]any{}
	}
	return Delivery{RequestID: eventRequestID, Result: frame.Data}, true, nil
}

func decodeTerminalError(eventRequestID string, line []byte) (Delivery, bool, error) {
	var frame errorFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
	}
	if frame.Code == "" || frame.Message == "" {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
	}
	delivery := Delivery{
		RequestID:    eventRequestID,
		ErrorCode:    frame.Code,
		ErrorMessage: frame.Message,
		ErrorDetails: cloneDetails(frame.Details),
	}
	return delivery, true, errorWithDetails(frame.Code, frame.Message, frame.Details, nil)
}
