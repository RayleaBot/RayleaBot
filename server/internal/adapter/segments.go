package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MessageSegment represents a structured message segment from the OneBot11
// protocol, normalized into a protocol-agnostic form.
type MessageSegment struct {
	Type string
	Data map[string]any
}

// parseCQString parses a OneBot11 CQ-coded message string into a slice of
// MessageSegment values. CQ codes take the form [CQ:type,key=value,...].
// Text between CQ codes becomes text segments.
func parseCQString(raw string) []MessageSegment {
	if raw == "" {
		return nil
	}

	var segments []MessageSegment
	remaining := raw

	for len(remaining) > 0 {
		idx := strings.Index(remaining, "[CQ:")
		if idx < 0 {
			text := unescapeCQ(remaining)
			if text != "" {
				segments = append(segments, MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
			break
		}

		if idx > 0 {
			text := unescapeCQ(remaining[:idx])
			if text != "" {
				segments = append(segments, MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
		}

		remaining = remaining[idx:]
		end := strings.Index(remaining, "]")
		if end < 0 {
			text := unescapeCQ(remaining)
			if text != "" {
				segments = append(segments, MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
			break
		}

		cqContent := remaining[4:end] // strip [CQ: and ]
		remaining = remaining[end+1:]

		seg := parseCQCode(cqContent)
		segments = append(segments, seg)
	}

	return segments
}

// parseCQCode parses the content inside [CQ:...] into a MessageSegment.
func parseCQCode(content string) MessageSegment {
	parts := strings.SplitN(content, ",", 2)
	cqType := strings.TrimSpace(parts[0])

	seg := MessageSegment{
		Type: normalizeCQType(cqType),
		Data: make(map[string]any),
	}

	if len(parts) < 2 {
		return seg
	}

	for _, param := range splitCQParams(parts[1]) {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := unescapeCQ(strings.TrimSpace(kv[1]))
		seg.Data[normalizeCQKey(cqType, key)] = value
	}

	// Normalize at with qq=all to at_all type.
	if seg.Type == "at" {
		if qq, ok := seg.Data["user_id"].(string); ok && qq == "all" {
			seg.Type = "at_all"
			delete(seg.Data, "user_id")
		}
	}

	return seg
}

// splitCQParams splits CQ parameters respecting that values may contain
// escaped commas.
func splitCQParams(s string) []string {
	var params []string
	var current strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == ',' {
			params = append(params, current.String())
			current.Reset()
			i++
			continue
		}
		current.WriteByte(s[i])
		i++
	}
	if current.Len() > 0 {
		params = append(params, current.String())
	}
	return params
}

// normalizeCQType maps OneBot11 CQ type names to unified segment types.
func normalizeCQType(cqType string) string {
	switch cqType {
	case "at":
		return "at"
	case "image":
		return "image"
	case "face":
		return "face"
	case "reply":
		return "reply"
	case "text":
		return "text"
	default:
		return cqType
	}
}

// normalizeCQKey maps OneBot11 CQ parameter keys to unified data keys.
func normalizeCQKey(cqType, key string) string {
	switch {
	case cqType == "at" && key == "qq":
		return "user_id"
	case cqType == "face" && key == "id":
		return "face_id"
	case cqType == "reply" && key == "id":
		return "message_id"
	default:
		return key
	}
}

// unescapeCQ reverses OneBot11 CQ code escape sequences.
func unescapeCQ(s string) string {
	s = strings.ReplaceAll(s, "&#44;", ",")
	s = strings.ReplaceAll(s, "&#91;", "[")
	s = strings.ReplaceAll(s, "&#93;", "]")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

// parseMessageArray parses a OneBot11 JSON message array into segments.
func parseMessageArray(raw json.RawMessage) ([]MessageSegment, error) {
	var items []struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	segments := make([]MessageSegment, 0, len(items))
	for _, item := range items {
		seg := MessageSegment{
			Type: normalizeCQType(item.Type),
			Data: make(map[string]any),
		}
		for k, v := range item.Data {
			seg.Data[normalizeCQKey(item.Type, k)] = v
		}
		// Normalize at with qq=all.
		if seg.Type == "at" {
			if qq, ok := seg.Data["user_id"]; ok {
				qqStr := anyToString(qq)
				if qqStr == "all" {
					seg.Type = "at_all"
					delete(seg.Data, "user_id")
				} else {
					seg.Data["user_id"] = qqStr
				}
			}
		}
		segments = append(segments, seg)
	}
	return segments, nil
}

// segmentsToPlainText generates a human-readable plain text representation
// from a slice of message segments.
func segmentsToPlainText(segments []MessageSegment) string {
	var b strings.Builder
	for _, seg := range segments {
		switch seg.Type {
		case "text":
			if text, ok := seg.Data["text"].(string); ok {
				b.WriteString(text)
			}
		case "image":
			b.WriteString("[图片]")
		case "at":
			b.WriteString("@某人")
		case "at_all":
			b.WriteString("@全体成员")
		case "face":
			b.WriteString("[表情]")
		case "reply":
			// Reply segments are omitted from plain text.
		default:
			b.WriteString("[未支持消息]")
		}
	}
	return b.String()
}

// anyToString converts a JSON-decoded value to a string representation.
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case json.Number:
		return val.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
