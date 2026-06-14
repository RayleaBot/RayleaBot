package action

import "strings"

func parseOutboundActionSegments(raw []protocolSegmentFrame) ([]ActionSegment, error) {
	if len(raw) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required rich message segments", nil)
	}

	segments := make([]ActionSegment, 0, len(raw))
	for index, segment := range raw {
		actionSegment, err := parseOutboundActionSegment(segment, index)
		if err != nil {
			return nil, err
		}
		segments = append(segments, actionSegment)
	}
	return segments, nil
}

func parseOutboundActionSegment(segment protocolSegmentFrame, index int) (ActionSegment, error) {
	segmentType := strings.TrimSpace(segment.Type)
	data := cloneActionSegmentData(segment.Data)

	switch segmentType {
	case "text":
		text, ok := data["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid text segment", nil)
		}
		data["text"] = text
	case "image":
		file := outboundActionString(data, "file")
		url := outboundActionString(data, "url")
		if file == "" && url == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid image segment", nil)
		}
		if file != "" {
			data["file"] = file
		}
		if url != "" {
			data["url"] = url
		}
	case "at":
		userID := outboundActionString(data, "user_id")
		if userID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid at segment", nil)
		}
		data["user_id"] = userID
	case "at_all":
		data = map[string]any{}
	case "face":
		faceID := outboundActionString(data, "face_id")
		if faceID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid face segment", nil)
		}
		data["face_id"] = faceID
	case "reply":
		if index != 0 {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame places reply segment outside the message head", nil)
		}
		messageID := outboundActionString(data, "message_id")
		if messageID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid reply segment", nil)
		}
		data["message_id"] = messageID
	case "record", "video", "file", "flash_file", "json", "xml", "markdown", "music", "contact", "forward", "node", "poke", "dice", "rps", "mface", "keyboard", "shake":
		if data == nil {
			data = map[string]any{}
		}
	default:
		return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported message segment type", nil)
	}

	return ActionSegment{
		Type: segmentType,
		Data: data,
	}, nil
}

func cloneActionSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func outboundActionString(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
