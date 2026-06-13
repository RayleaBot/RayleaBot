package runtime

func buildProtocolBilibiliPayload(fields map[string]any) (*protocolBilibiliPayloadFrame, bool) {
	raw, ok := fields["bilibili"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	kind, hasKind := payloadString(raw, "kind")
	uid, hasUID := payloadString(raw, "uid")
	id, hasID := payloadString(raw, "id")
	service, hasService := payloadString(raw, "service")
	url, hasURL := payloadString(raw, "url")
	author, hasAuthor := buildProtocolBilibiliAuthor(raw)
	if !hasKind || !hasUID || !hasID || !hasService || !hasURL || !hasAuthor {
		return nil, false
	}

	payload := protocolBilibiliPayloadFrame{
		Kind:    kind,
		UID:     uid,
		ID:      id,
		Service: service,
		URL:     url,
		Author:  author,
	}
	if v, ok := payloadString(raw, "room_id"); ok {
		payload.RoomID = v
	}
	if v, ok := payloadString(raw, "title"); ok {
		payload.Title = v
	}
	if v, ok := payloadString(raw, "summary"); ok {
		payload.Summary = v
	}
	if v, ok := payloadString(raw, "summary_html"); ok {
		payload.SummaryHTML = v
	}
	if v, ok := payloadInt64(raw, "pub_ts"); ok {
		payload.PubTS = v
	}
	if v, ok := payloadString(raw, "created_at"); ok {
		payload.CreatedAt = v
	}
	if images := buildProtocolBilibiliImages(raw); len(images) > 0 {
		payload.Images = images
	}
	if topic, ok := buildProtocolBilibiliTopic(raw); ok {
		payload.Topic = topic
	}
	if original, ok := buildProtocolBilibiliOriginal(raw); ok {
		payload.Original = original
	}
	if v, ok := payloadIntAllowZero(raw, "live_status"); ok {
		payload.LiveStatus = &v
	}
	if v, ok := payloadString(raw, "live_event"); ok {
		payload.LiveEvent = v
	}
	if v, ok := payloadString(raw, "status_label"); ok {
		payload.StatusLabel = v
	}
	if v, ok := payloadString(raw, "live_started_at"); ok {
		payload.LiveStartedAt = v
	}
	if v, ok := payloadString(raw, "live_detected_at"); ok {
		payload.LiveDetectedAt = v
	}
	if v, ok := payloadString(raw, "dynamic_type"); ok {
		payload.DynamicType = v
	}
	return &payload, true
}
