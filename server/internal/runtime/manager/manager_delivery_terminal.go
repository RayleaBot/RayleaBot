package manager

import (
	"encoding/json"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func parseEventEnvelope(line []byte, pluginID string) (runtimeprotocol.FrameEnvelope, error) {
	var envelope runtimeprotocol.FrameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return runtimeprotocol.FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return runtimeprotocol.FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return runtimeprotocol.FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" {
		return runtimeprotocol.FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
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
	var frame runtimeprotocol.ActionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}
	action, err := runtimeaction.ParseTerminalAction(frame.Action, frame.Data)
	if err != nil {
		return Delivery{}, false, normalizeRuntimeError(err, "parse terminal action frame")
	}
	return Delivery{RequestID: eventRequestID, Action: action}, true, nil
}

func decodeTerminalResult(eventRequestID string, line []byte) (Delivery, bool, error) {
	var frame runtimeprotocol.ResultFrame
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
	var frame runtimeprotocol.ErrorFrame
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
