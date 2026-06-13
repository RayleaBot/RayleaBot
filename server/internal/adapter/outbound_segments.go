package adapter

import "strings"

func validateOutboundTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", errorf(errorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", errorf(errorCodeSendFailed, actionKind+" uses unsupported target_type", nil)
	}
}

func (s *Shell) normalizeOutboundSegments(actionKind string, declared []OutboundMessageSegment, replyToMessageID string) ([]oneBotMessageSegment, error) {
	segments := make([]OutboundMessageSegment, 0, len(declared)+1)
	for _, segment := range declared {
		segments = append(segments, OutboundMessageSegment{
			Type: segment.Type,
			Data: cloneOutboundSegmentData(segment.Data),
		})
	}
	if len(segments) == 0 {
		return nil, errorf(errorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	if replyToMessageID != "" {
		reply := OutboundMessageSegment{
			Type: "reply",
			Data: map[string]any{"message_id": replyToMessageID},
		}
		segments = prependReplySegment(segments, reply)
	}

	converted := make([]oneBotMessageSegment, 0, len(segments))
	for _, segment := range segments {
		oneBotSegment, ok := convertOutboundSegment(segment)
		if !ok {
			s.logger.Warn(
				"dropping unsupported outbound message segment",
				"component", "adapter",
				"segment_type", segment.Type,
			)
			continue
		}
		converted = append(converted, oneBotSegment)
	}
	if len(converted) == 0 {
		return nil, errorf(errorCodeSendFailed, "outbound message became empty after segment normalization", nil)
	}
	return converted, nil
}

func prependReplySegment(segments []OutboundMessageSegment, reply OutboundMessageSegment) []OutboundMessageSegment {
	result := make([]OutboundMessageSegment, 0, len(segments)+1)
	result = append(result, reply)
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) == "reply" {
			continue
		}
		result = append(result, segment)
	}
	return result
}

func convertOutboundSegment(segment OutboundMessageSegment) (oneBotMessageSegment, bool) {
	switch strings.TrimSpace(segment.Type) {
	case "text":
		text, ok := outboundSegmentString(segment.Data, "text")
		if !ok || text == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "text",
			Data: map[string]any{"text": text},
		}, true
	case "image":
		if file, ok := outboundSegmentString(segment.Data, "file"); ok && file != "" {
			return oneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": file},
			}, true
		}
		if url, ok := outboundSegmentString(segment.Data, "url"); ok && url != "" {
			return oneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": url},
			}, true
		}
		return oneBotMessageSegment{}, false
	case "at":
		userID, ok := outboundSegmentString(segment.Data, "user_id")
		if !ok || userID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": userID},
		}, true
	case "at_all":
		return oneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": "all"},
		}, true
	case "face":
		faceID, ok := outboundSegmentString(segment.Data, "face_id")
		if !ok || faceID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "face",
			Data: map[string]any{"id": faceID},
		}, true
	case "reply":
		messageID, ok := outboundSegmentString(segment.Data, "message_id")
		if !ok || messageID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "reply",
			Data: map[string]any{"id": messageID},
		}, true
	case "record", "video", "file", "json", "xml", "markdown", "music", "contact", "forward", "node", "poke", "dice", "rps", "mface", "keyboard", "shake":
		return oneBotMessageSegment{
			Type: strings.TrimSpace(segment.Type),
			Data: cloneOutboundSegmentData(segment.Data),
		}, true
	default:
		return oneBotMessageSegment{}, false
	}
}

func cloneOutboundSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func outboundSegmentString(data map[string]any, key string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}
