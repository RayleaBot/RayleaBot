package monitoring

import "strings"

func Payload(event Event) map[string]any {
	payload := map[string]any{
		"kind":    event.Kind,
		"uid":     event.UID,
		"id":      event.ID,
		"service": event.Service,
		"url":     event.URL,
		"author": map[string]any{
			"uid":    event.Author.UID,
			"name":   event.Author.Name,
			"avatar": event.Author.Avatar,
		},
	}
	putString(payload, "room_id", event.RoomID)
	putString(payload, "title", event.Title)
	putString(payload, "summary", event.Summary)
	putString(payload, "summary_html", event.SummaryHTML)
	putString(payload, "created_at", event.CreatedAt)
	putString(payload, "live_event", event.LiveEvent)
	putString(payload, "status_label", event.StatusLabel)
	putString(payload, "live_started_at", event.LiveStartedAt)
	putString(payload, "live_detected_at", event.LiveDetectedAt)
	putString(payload, "dynamic_type", event.DynamicType)
	if event.PubTS > 0 {
		payload["pub_ts"] = event.PubTS
	}
	if event.LiveStatus != nil {
		payload["live_status"] = *event.LiveStatus
	}
	if images := imagesPayload(event.Images); len(images) > 0 {
		payload["images"] = images
	}
	if topic := topicPayload(event.Topic); topic != nil {
		payload["topic"] = topic
	}
	if original := originalPayload(event.Original); original != nil {
		payload["original"] = original
	}
	return payload
}

func originalPayload(original *Original) map[string]any {
	if original == nil || original.ID == "" || original.Service == "" || original.URL == "" || original.Author.UID == "" || original.Author.Name == "" {
		return nil
	}
	payload := map[string]any{
		"id":      original.ID,
		"service": original.Service,
		"url":     original.URL,
		"author": map[string]any{
			"uid":    original.Author.UID,
			"name":   original.Author.Name,
			"avatar": original.Author.Avatar,
		},
	}
	putString(payload, "title", original.Title)
	putString(payload, "summary", original.Summary)
	putString(payload, "summary_html", original.SummaryHTML)
	putString(payload, "created_at", original.CreatedAt)
	putString(payload, "dynamic_type", original.DynamicType)
	if original.PubTS > 0 {
		payload["pub_ts"] = original.PubTS
	}
	if images := imagesPayload(original.Images); len(images) > 0 {
		payload["images"] = images
	}
	if topic := topicPayload(original.Topic); topic != nil {
		payload["topic"] = topic
	}
	return payload
}

func topicPayload(topic *Topic) map[string]any {
	if topic == nil || strings.TrimSpace(topic.Name) == "" {
		return nil
	}
	payload := map[string]any{
		"name": topic.Name,
	}
	if topic.ID > 0 {
		payload["id"] = topic.ID
	}
	putString(payload, "jump_url", topic.JumpURL)
	return payload
}

func imagesPayload(source []Image) []map[string]any {
	if len(source) == 0 {
		return nil
	}
	images := make([]map[string]any, 0, len(source))
	for _, image := range source {
		if image.URL == "" {
			continue
		}
		item := map[string]any{"url": image.URL}
		if image.Width > 0 {
			item["width"] = image.Width
		}
		if image.Height > 0 {
			item["height"] = image.Height
		}
		images = append(images, item)
	}
	return images
}

func putString(values map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		values[key] = value
	}
}
