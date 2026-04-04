package runtime

import (
	"context"
	"encoding/json"
)

func buildEventFrame(event Event, pluginID string, requestID string, timestamp int64) eventFrame {
	frame := eventFrame{
		ProtocolVersion: "1",
		Type:            "event",
		Timestamp:       timestamp,
		PluginID:        pluginID,
		RequestID:       requestID,
		Event: protocolEventFrame{
			EventID:        event.EventID,
			SourceProtocol: event.SourceProtocol,
			SourceAdapter:  event.SourceAdapter,
			EventType:      event.EventType,
			Timestamp:      event.Timestamp,
		},
	}
	if event.Actor != nil && event.Actor.ID != "" {
		frame.Event.Actor = &protocolActorFrame{
			ID:       event.Actor.ID,
			Nickname: event.Actor.Nickname,
			Role:     event.Actor.Role,
		}
	}
	if event.Target != nil && event.Target.Type != "" && event.Target.ID != "" {
		frame.Event.Target = &protocolTargetFrame{
			Type: event.Target.Type,
			ID:   event.Target.ID,
			Name: event.Target.Name,
		}
	}
	if event.Message != nil && event.Message.PlainText != "" {
		msgFrame := &protocolMessageFrame{PlainText: event.Message.PlainText}
		for _, seg := range event.Message.Segments {
			msgFrame.Segments = append(msgFrame.Segments, protocolSegmentFrame{
				Type: seg.Type,
				Data: seg.Data,
			})
		}
		frame.Event.Message = msgFrame
	}
	if payload, ok := buildEventPayload(event); ok {
		frame.Event.Payload = payload
	}
	if event.RawPayload != nil {
		frame.Event.RawPayload = event.RawPayload
	}
	return frame
}

func buildEventPayload(event Event) (*protocolPayloadFrame, bool) {
	var payload protocolPayloadFrame
	hasPayload := false
	if event.MessageID != "" {
		payload.MessageID = event.MessageID
		hasPayload = true
	}
	if event.PayloadFields != nil {
		if v, ok := event.PayloadFields["sub_type"].(string); ok && v != "" {
			payload.SubType = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["operator_id"].(string); ok && v != "" {
			payload.OperatorID = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["command"].(string); ok && v != "" {
			payload.Command = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["args"].([]string); ok && len(v) > 0 {
			payload.Args = v
			hasPayload = true
		}
	}
	if !hasPayload {
		return nil, false
	}
	return &payload, true
}

func (m *Manager) processEventFrame(ctx context.Context, handle *processHandle, eventRequestID string, seenLocalRequestIDs map[string]struct{}, line []byte) (Delivery, bool, error) {
	envelope, err := parseEventEnvelope(line, handle.spec.PluginID)
	if err != nil {
		return Delivery{}, false, err
	}
	if envelope.RequestID != eventRequestID {
		if err := m.handleLocalActionFrame(ctx, handle, envelope, seenLocalRequestIDs, line); err != nil {
			return Delivery{}, false, err
		}
		return Delivery{}, false, nil
	}
	return decodeTerminalDelivery(eventRequestID, line, envelope.Type)
}

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
	case "logger.write", "storage.kv", "config.read", "config.write", "storage.file", "http.request", "scheduler.create", "event.expose_webhook", "render.image":
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
	default:
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
	}
	return delivery, true, errorf(frame.Code, frame.Message, nil)
}
