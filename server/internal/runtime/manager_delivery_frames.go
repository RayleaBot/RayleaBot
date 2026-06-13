package runtime

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
