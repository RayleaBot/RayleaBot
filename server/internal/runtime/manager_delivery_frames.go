package runtime

import "encoding/json"

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
	if event.Message != nil && (event.Message.PlainText != "" || len(event.Message.Segments) > 0) {
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
		if onebot, ok := buildProtocolOneBotPayload(event.PayloadFields); ok {
			payload.OneBot = onebot
			hasPayload = true
		}
	}
	if !hasPayload {
		return nil, false
	}
	return &payload, true
}

func buildProtocolOneBotPayload(fields map[string]any) (*protocolOneBotPayloadFrame, bool) {
	raw, ok := fields["onebot"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}

	var payload protocolOneBotPayloadFrame
	hasPayload := false
	if v, ok := payloadString(raw, "post_type"); ok {
		payload.PostType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "meta_event_type"); ok {
		payload.MetaEventType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "message_type"); ok {
		payload.MessageType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "request_type"); ok {
		payload.RequestType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "notice_type"); ok {
		payload.NoticeType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "sub_type"); ok {
		payload.SubType = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "self_id"); ok {
		payload.SelfID = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "user_id"); ok {
		payload.UserID = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "group_id"); ok {
		payload.GroupID = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "target_id"); ok {
		payload.TargetID = v
		hasPayload = true
	}
	if v, ok := payloadInt64(raw, "time"); ok {
		payload.Time = v
		hasPayload = true
	}
	if v, ok := payloadInt(raw, "interval"); ok {
		payload.Interval = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "message_id"); ok {
		payload.MessageID = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "real_id"); ok {
		payload.RealID = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "message_seq"); ok {
		payload.MessageSeq = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "raw_message"); ok {
		payload.RawMessage = v
		hasPayload = true
	}
	if v, ok := payloadInt(raw, "font"); ok {
		payload.Font = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "message_format"); ok {
		payload.MessageFormat = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "comment"); ok {
		payload.Comment = v
		hasPayload = true
	}
	if v, ok := payloadString(raw, "flag"); ok {
		payload.Flag = v
		hasPayload = true
	}
	if v, ok := payloadMap(raw, "status"); ok {
		payload.Status = v
		hasPayload = true
	}
	if sender, ok := buildProtocolOneBotSender(raw); ok {
		payload.Sender = sender
		hasPayload = true
	}
	if !hasPayload {
		return nil, false
	}
	return &payload, true
}

func buildProtocolOneBotSender(raw map[string]any) (*protocolOneBotSenderFrame, bool) {
	senderRaw, ok := raw["sender"].(map[string]any)
	if !ok || len(senderRaw) == 0 {
		return nil, false
	}

	var sender protocolOneBotSenderFrame
	hasPayload := false
	if v, ok := payloadString(senderRaw, "user_id"); ok {
		sender.UserID = v
		hasPayload = true
	}
	if v, ok := payloadString(senderRaw, "nickname"); ok {
		sender.Nickname = v
		hasPayload = true
	}
	if v, ok := payloadString(senderRaw, "card"); ok {
		sender.Card = v
		hasPayload = true
	}
	if v, ok := payloadString(senderRaw, "role"); ok {
		sender.Role = v
		hasPayload = true
	}
	if v, ok := payloadString(senderRaw, "title"); ok {
		sender.Title = v
		hasPayload = true
	}
	if v, ok := payloadString(senderRaw, "sex"); ok {
		sender.Sex = v
		hasPayload = true
	}
	if v, ok := payloadInt(senderRaw, "age"); ok {
		sender.Age = v
		hasPayload = true
	}
	if !hasPayload {
		return nil, false
	}
	return &sender, true
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
