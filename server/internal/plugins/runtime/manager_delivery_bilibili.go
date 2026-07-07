package runtime

func buildProtocolBilibiliPayload(fields map[string]any) (*ProtocolBilibiliPayloadFrame, bool) {
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

	payload := ProtocolBilibiliPayloadFrame{
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

func buildProtocolBilibiliOriginal(raw map[string]any) (*ProtocolBilibiliOriginalFrame, bool) {
	originalRaw, ok := raw["original"].(map[string]any)
	if !ok || len(originalRaw) == 0 {
		return nil, false
	}
	id, hasID := payloadString(originalRaw, "id")
	service, hasService := payloadString(originalRaw, "service")
	url, hasURL := payloadString(originalRaw, "url")
	author, hasAuthor := buildProtocolBilibiliAuthor(originalRaw)
	if !hasID || !hasService || !hasURL || !hasAuthor {
		return nil, false
	}
	original := ProtocolBilibiliOriginalFrame{
		ID:      id,
		Service: service,
		URL:     url,
		Author:  author,
	}
	if v, ok := payloadString(originalRaw, "title"); ok {
		original.Title = v
	}
	if v, ok := payloadString(originalRaw, "summary"); ok {
		original.Summary = v
	}
	if v, ok := payloadString(originalRaw, "summary_html"); ok {
		original.SummaryHTML = v
	}
	if v, ok := payloadInt64(originalRaw, "pub_ts"); ok {
		original.PubTS = v
	}
	if v, ok := payloadString(originalRaw, "created_at"); ok {
		original.CreatedAt = v
	}
	if images := buildProtocolBilibiliImages(originalRaw); len(images) > 0 {
		original.Images = images
	}
	if topic, ok := buildProtocolBilibiliTopic(originalRaw); ok {
		original.Topic = topic
	}
	if v, ok := payloadString(originalRaw, "dynamic_type"); ok {
		original.DynamicType = v
	}
	return &original, true
}

func buildProtocolBilibiliTopic(raw map[string]any) (*ProtocolBilibiliTopicFrame, bool) {
	topicRaw, ok := raw["topic"].(map[string]any)
	if !ok || len(topicRaw) == 0 {
		return nil, false
	}
	name, hasName := payloadString(topicRaw, "name")
	if !hasName {
		return nil, false
	}
	topic := ProtocolBilibiliTopicFrame{Name: name}
	if id, ok := payloadInt64(topicRaw, "id"); ok {
		topic.ID = id
	}
	if jumpURL, ok := payloadString(topicRaw, "jump_url"); ok {
		topic.JumpURL = jumpURL
	}
	return &topic, true
}

func buildProtocolBilibiliAuthor(raw map[string]any) (ProtocolBilibiliAuthorFrame, bool) {
	authorRaw, ok := raw["author"].(map[string]any)
	if !ok || len(authorRaw) == 0 {
		return ProtocolBilibiliAuthorFrame{}, false
	}
	uid, hasUID := payloadString(authorRaw, "uid")
	name, hasName := payloadString(authorRaw, "name")
	if !hasUID || !hasName {
		return ProtocolBilibiliAuthorFrame{}, false
	}
	author := ProtocolBilibiliAuthorFrame{UID: uid, Name: name}
	if avatar, ok := payloadString(authorRaw, "avatar"); ok {
		author.Avatar = avatar
	}
	return author, true
}

func buildProtocolBilibiliImages(raw map[string]any) []ProtocolBilibiliImageFrame {
	source, ok := raw["images"].([]map[string]any)
	if !ok {
		sourceAny, ok := raw["images"].([]any)
		if !ok {
			return nil
		}
		source = make([]map[string]any, 0, len(sourceAny))
		for _, item := range sourceAny {
			if image, ok := item.(map[string]any); ok {
				source = append(source, image)
			}
		}
	}
	images := make([]ProtocolBilibiliImageFrame, 0, len(source))
	for _, item := range source {
		url, ok := payloadString(item, "url")
		if !ok {
			continue
		}
		image := ProtocolBilibiliImageFrame{URL: url}
		if width, ok := payloadIntAllowZero(item, "width"); ok {
			image.Width = width
		}
		if height, ok := payloadIntAllowZero(item, "height"); ok {
			image.Height = height
		}
		images = append(images, image)
	}
	return images
}
