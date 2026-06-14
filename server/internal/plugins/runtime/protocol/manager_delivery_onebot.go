package protocol

func buildProtocolOneBotPayload(fields map[string]any) (*ProtocolOneBotPayloadFrame, bool) {
	raw, ok := fields["onebot"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}

	var payload ProtocolOneBotPayloadFrame
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

func buildProtocolOneBotSender(raw map[string]any) (*ProtocolOneBotSenderFrame, bool) {
	senderRaw, ok := raw["sender"].(map[string]any)
	if !ok || len(senderRaw) == 0 {
		return nil, false
	}

	var sender ProtocolOneBotSenderFrame
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
