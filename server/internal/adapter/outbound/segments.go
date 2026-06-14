package outbound

import "strings"

func ValidateTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", Errorf(ErrorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", Errorf(ErrorCodeSendFailed, actionKind+" uses unsupported target_type", nil)
	}
}

type NormalizedSegments struct {
	Segments           []OneBotMessageSegment
	UnsupportedSegment []string
}

func NormalizeSegments(actionKind string, declared []OutboundMessageSegment, replyToMessageID string) (NormalizedSegments, error) {
	segments := make([]OutboundMessageSegment, 0, len(declared)+1)
	for _, segment := range declared {
		segments = append(segments, OutboundMessageSegment{
			Type: segment.Type,
			Data: cloneOutboundSegmentData(segment.Data),
		})
	}
	if len(segments) == 0 {
		return NormalizedSegments{}, Errorf(ErrorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	if replyToMessageID != "" {
		reply := OutboundMessageSegment{
			Type: "reply",
			Data: map[string]any{"message_id": replyToMessageID},
		}
		segments = prependReplySegment(segments, reply)
	}

	converted := make([]OneBotMessageSegment, 0, len(segments))
	unsupported := make([]string, 0)
	for _, segment := range segments {
		oneBotSegment, ok := convertOutboundSegment(segment)
		if !ok {
			unsupported = append(unsupported, strings.TrimSpace(segment.Type))
			continue
		}
		converted = append(converted, oneBotSegment)
	}
	if len(converted) == 0 {
		return NormalizedSegments{}, Errorf(ErrorCodeSendFailed, "outbound message became empty after segment normalization", nil)
	}
	return NormalizedSegments{
		Segments:           converted,
		UnsupportedSegment: unsupported,
	}, nil
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

func convertOutboundSegment(segment OutboundMessageSegment) (OneBotMessageSegment, bool) {
	switch strings.TrimSpace(segment.Type) {
	case "text":
		text, ok := outboundSegmentString(segment.Data, "text")
		if !ok || text == "" {
			return OneBotMessageSegment{}, false
		}
		return OneBotMessageSegment{
			Type: "text",
			Data: map[string]any{"text": text},
		}, true
	case "image":
		if file, ok := outboundSegmentString(segment.Data, "file"); ok && file != "" {
			return OneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": file},
			}, true
		}
		if url, ok := outboundSegmentString(segment.Data, "url"); ok && url != "" {
			return OneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": url},
			}, true
		}
		return OneBotMessageSegment{}, false
	case "at":
		userID, ok := outboundSegmentString(segment.Data, "user_id")
		if !ok || userID == "" {
			return OneBotMessageSegment{}, false
		}
		return OneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": userID},
		}, true
	case "at_all":
		return OneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": "all"},
		}, true
	case "face":
		faceID, ok := outboundSegmentString(segment.Data, "face_id")
		if !ok || faceID == "" {
			return OneBotMessageSegment{}, false
		}
		return OneBotMessageSegment{
			Type: "face",
			Data: map[string]any{"id": faceID},
		}, true
	case "reply":
		messageID, ok := outboundSegmentString(segment.Data, "message_id")
		if !ok || messageID == "" {
			return OneBotMessageSegment{}, false
		}
		return OneBotMessageSegment{
			Type: "reply",
			Data: map[string]any{"id": messageID},
		}, true
	case "record", "video", "file", "json", "xml", "markdown", "music", "contact", "forward", "node", "poke", "dice", "rps", "mface", "keyboard", "shake":
		return OneBotMessageSegment{
			Type: strings.TrimSpace(segment.Type),
			Data: cloneOutboundSegmentData(segment.Data),
		}, true
	default:
		return OneBotMessageSegment{}, false
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
