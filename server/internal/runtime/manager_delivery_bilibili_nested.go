package runtime

func buildProtocolBilibiliOriginal(raw map[string]any) (*protocolBilibiliOriginalFrame, bool) {
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
	original := protocolBilibiliOriginalFrame{
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

func buildProtocolBilibiliTopic(raw map[string]any) (*protocolBilibiliTopicFrame, bool) {
	topicRaw, ok := raw["topic"].(map[string]any)
	if !ok || len(topicRaw) == 0 {
		return nil, false
	}
	name, hasName := payloadString(topicRaw, "name")
	if !hasName {
		return nil, false
	}
	topic := protocolBilibiliTopicFrame{Name: name}
	if id, ok := payloadInt64(topicRaw, "id"); ok {
		topic.ID = id
	}
	if jumpURL, ok := payloadString(topicRaw, "jump_url"); ok {
		topic.JumpURL = jumpURL
	}
	return &topic, true
}

func buildProtocolBilibiliAuthor(raw map[string]any) (protocolBilibiliAuthorFrame, bool) {
	authorRaw, ok := raw["author"].(map[string]any)
	if !ok || len(authorRaw) == 0 {
		return protocolBilibiliAuthorFrame{}, false
	}
	uid, hasUID := payloadString(authorRaw, "uid")
	name, hasName := payloadString(authorRaw, "name")
	if !hasUID || !hasName {
		return protocolBilibiliAuthorFrame{}, false
	}
	author := protocolBilibiliAuthorFrame{UID: uid, Name: name}
	if avatar, ok := payloadString(authorRaw, "avatar"); ok {
		author.Avatar = avatar
	}
	return author, true
}

func buildProtocolBilibiliImages(raw map[string]any) []protocolBilibiliImageFrame {
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
	images := make([]protocolBilibiliImageFrame, 0, len(source))
	for _, item := range source {
		url, ok := payloadString(item, "url")
		if !ok {
			continue
		}
		image := protocolBilibiliImageFrame{URL: url}
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
