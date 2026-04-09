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
	case "contact":
		return "contact"
	case "dice":
		return "dice"
	case "image":
		return "image"
	case "face":
		return "face"
	case "file":
		return "file"
	case "flash", "flash_file":
		return "flash_file"
	case "forward":
		return "forward"
	case "json":
		return "json"
	case "keyboard":
		return "keyboard"
	case "markdown":
		return "markdown"
	case "mface":
		return "mface"
	case "music":
		return "music"
	case "node":
		return "node"
	case "poke":
		return "poke"
	case "record":
		return "record"
	case "reply":
		return "reply"
	case "rps":
		return "rps"
	case "shake":
		return "shake"
	case "text":
		return "text"
	case "video":
		return "video"
	case "xml":
		return "xml"
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
	case cqType == "contact" && key == "id":
		return "target_id"
	case cqType == "contact" && key == "type":
		return "target_type"
	case (cqType == "flash" || cqType == "flash_file") && key == "file":
		return "file"
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
		case "video":
			b.WriteString("[视频]")
		case "record":
			b.WriteString("[语音]")
		case "file":
			b.WriteString(segmentLabel(seg.Data, "name", "file", "[文件]"))
		case "flash_file":
			b.WriteString(segmentLabel(seg.Data, "name", "file", "[闪传文件]"))
		case "at":
			b.WriteString(segmentMention(seg.Data))
		case "at_all":
			b.WriteString("@全体成员")
		case "face":
			b.WriteString("[表情]")
		case "mface":
			b.WriteString("[大表情]")
		case "reply":
			// Reply segments are omitted from plain text.
		case "json":
			b.WriteString("[卡片消息]")
		case "xml":
			b.WriteString("[XML消息]")
		case "markdown":
			if text, ok := seg.Data["text"].(string); ok && strings.TrimSpace(text) != "" {
				b.WriteString(text)
			} else {
				b.WriteString("[Markdown消息]")
			}
		case "music":
			b.WriteString("[音乐卡片]")
		case "contact":
			b.WriteString("[名片]")
		case "forward":
			b.WriteString("[合并转发]")
		case "node":
			b.WriteString("[转发节点]")
		case "poke":
			b.WriteString("[戳一戳]")
		case "dice":
			b.WriteString("[骰子]")
		case "rps":
			b.WriteString("[猜拳]")
		case "keyboard":
			b.WriteString("[按键面板]")
		case "keyboard_button":
			b.WriteString("[按钮]")
		case "shake":
			b.WriteString("[窗口抖动]")
		default:
			b.WriteString("[未支持消息]")
		}
	}
	return b.String()
}

func segmentMention(data map[string]any) string {
	userID := anyToString(data["user_id"])
	if userID == "" {
		return "@某人"
	}
	return "@" + userID
}

func segmentLabel(data map[string]any, preferredKey string, fallbackKey string, empty string) string {
	if text := strings.TrimSpace(anyToString(data[preferredKey])); text != "" && text != "null" {
		return empty[:len(empty)-1] + ":" + text + "]"
	}
	if text := strings.TrimSpace(anyToString(data[fallbackKey])); text != "" && text != "null" {
		return empty[:len(empty)-1] + ":" + text + "]"
	}
	return empty
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
