package runtime

import (
	"context"
	"encoding/json"
	"time"
)

func (m *Manager) DeliverEvent(ctx context.Context, event Event) (Delivery, error) {
	if event.EventID == "" || event.SourceProtocol == "" || event.SourceAdapter == "" || event.EventType == "" || event.Timestamp <= 0 {
		return Delivery{}, errorf(codePlatformInvalidRequest, "event payload is missing required fields", nil)
	}

	m.mu.RLock()
	handle := m.proc
	m.mu.RUnlock()
	if handle == nil {
		return Delivery{}, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}

	requestID := m.deps.requestID()
	session, runtimeErr := m.registerEventSession(ctx, handle, requestID, event)
	if runtimeErr != nil {
		return Delivery{}, runtimeErr
	}

	frame := BuildEventFrame(event, handle.Spec.PluginID, requestID, m.deps.now().Unix())
	if err := handle.WriteJSONLine(frame); err != nil {
		m.removeEventSession(handle, requestID)
		return Delivery{}, m.failRuntime(handle, codePluginInternalError, "write event frame", err)
	}

	timeout := handle.Spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-session.done:
		if session.err != nil {
			return session.delivery, session.err
		}
		return session.delivery, nil
	case <-timer.C:
		return m.timeoutEvent(handle, session, codePluginEventTimeout, "plugin event response timed out", nil)
	case <-ctx.Done():
		return m.timeoutEvent(handle, session, codePluginEventTimeout, "plugin event response timed out", ctx.Err())
	}
}

func BuildEventFrame(event Event, pluginID string, requestID string, timestamp int64) EventFrame {
	frame := EventFrame{
		ProtocolVersion: "1",
		Type:            "event",
		Timestamp:       timestamp,
		PluginID:        pluginID,
		RequestID:       requestID,
		Event: ProtocolEventFrame{
			EventID:        event.EventID,
			SourceProtocol: event.SourceProtocol,
			SourceAdapter:  event.SourceAdapter,
			EventType:      event.EventType,
			Timestamp:      event.Timestamp,
		},
	}
	if event.Actor != nil && event.Actor.ID != "" {
		frame.Event.Actor = &ProtocolActorFrame{
			ID:       event.Actor.ID,
			Nickname: event.Actor.Nickname,
			Role:     event.Actor.Role,
		}
	}
	if event.Target != nil && event.Target.Type != "" && event.Target.ID != "" {
		frame.Event.Target = &ProtocolTargetFrame{
			Type: event.Target.Type,
			ID:   event.Target.ID,
			Name: event.Target.Name,
		}
	}
	if event.Message != nil && (event.Message.PlainText != "" || len(event.Message.Segments) > 0) {
		msgFrame := &ProtocolMessageFrame{PlainText: event.Message.PlainText}
		for _, seg := range event.Message.Segments {
			msgFrame.Segments = append(msgFrame.Segments, ProtocolSegmentFrame{
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

func buildEventPayload(event Event) (*ProtocolPayloadFrame, bool) {
	var payload ProtocolPayloadFrame
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
		if onebot, ok := buildProtocolOneBotPayload(event.PayloadFields); ok {
			payload.OneBot = onebot
			hasPayload = true
		}
		if bilibili, ok := buildProtocolBilibiliPayload(event.PayloadFields); ok {
			payload.Bilibili = bilibili
			hasPayload = true
		}
	}
	if !hasPayload {
		return nil, false
	}
	return &payload, true
}

func payloadString(values map[string]any, key string) (string, bool) {
	value, ok := values[key].(string)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func payloadInt64(values map[string]any, key string) (int64, bool) {
	switch value := values[key].(type) {
	case int64:
		if value <= 0 {
			return 0, false
		}
		return value, true
	case int:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	case float64:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}

func payloadInt(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		if value <= 0 {
			return 0, false
		}
		return value, true
	case int64:
		if value <= 0 {
			return 0, false
		}
		return int(value), true
	case float64:
		if value <= 0 {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func payloadIntAllowZero(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		if value < 0 {
			return 0, false
		}
		return value, true
	case int64:
		if value < 0 {
			return 0, false
		}
		return int(value), true
	case float64:
		if value < 0 {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func payloadMap(values map[string]any, key string) (map[string]any, bool) {
	raw, ok := values[key].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	cloned := make(map[string]any, len(raw))
	for mapKey, value := range raw {
		cloned[mapKey] = value
	}
	return cloned, true
}

func parseEventEnvelope(line []byte, pluginID string) (FrameEnvelope, error) {
	var envelope FrameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" {
		return FrameEnvelope{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
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
	var frame ActionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}
	action, err := ParseTerminalAction(frame.Action, frame.Data)
	if err != nil {
		return Delivery{}, false, normalizeRuntimeError(err, "parse terminal action frame")
	}
	return Delivery{RequestID: eventRequestID, Action: action}, true, nil
}

func decodeTerminalResult(eventRequestID string, line []byte) (Delivery, bool, error) {
	var frame ResultFrame
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
	var frame ErrorFrame
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
