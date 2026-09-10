package onebot11

import (
	"encoding/json"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/redact"
)

// parseMessageArray parses a OneBot11 JSON message array into segments.
func parseMessageArray(raw json.RawMessage) ([]chatevent.MessageSegment, error) {
	var items []struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	segments := make([]chatevent.MessageSegment, 0, len(items))
	for _, item := range items {
		seg := chatevent.MessageSegment{
			Type: normalizeCQType(item.Type),
			Data: make(map[string]any),
		}
		for k, v := range item.Data {
			seg.Data[normalizeCQKey(item.Type, k)] = redact.SanitizeAny(v)
		}
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

// ParseMessageArray parses a OneBot11 JSON message array into segments.
func ParseMessageArray(raw json.RawMessage) ([]chatevent.MessageSegment, error) {
	return parseMessageArray(raw)
}

// anyToString converts a JSON-decoded value to a string representation.
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return redact.SanitizeString(val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case json.Number:
		return val.String()
	default:
		b, _ := json.Marshal(v)
		return redact.SanitizeString(string(b))
	}
}

// parseCQString parses a OneBot11 CQ-coded message string into a slice of
// chatevent.MessageSegment values. CQ codes take the form [CQ:type,key=value,...].
// Text between CQ codes becomes text segments.
func parseCQString(raw string) []chatevent.MessageSegment {
	if raw == "" {
		return nil
	}

	var segments []chatevent.MessageSegment
	remaining := raw

	for len(remaining) > 0 {
		idx := strings.Index(remaining, "[CQ:")
		if idx < 0 {
			text := redact.SanitizeString(unescapeCQ(remaining))
			if text != "" {
				segments = append(segments, chatevent.MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
			break
		}

		if idx > 0 {
			text := redact.SanitizeString(unescapeCQ(remaining[:idx]))
			if text != "" {
				segments = append(segments, chatevent.MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
		}

		remaining = remaining[idx:]
		end := strings.Index(remaining, "]")
		if end < 0 {
			text := redact.SanitizeString(unescapeCQ(remaining))
			if text != "" {
				segments = append(segments, chatevent.MessageSegment{
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

// ParseCQString parses a OneBot11 CQ-coded message string.
func ParseCQString(raw string) []chatevent.MessageSegment {
	return parseCQString(raw)
}

// parseCQCode parses the content inside [CQ:...] into a chatevent.MessageSegment.
func parseCQCode(content string) chatevent.MessageSegment {
	parts := strings.SplitN(content, ",", 2)
	cqType := strings.TrimSpace(parts[0])

	seg := chatevent.MessageSegment{
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
		value := redact.SanitizeString(unescapeCQ(strings.TrimSpace(kv[1])))
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

// unescapeCQ reverses OneBot11 CQ code escape sequences.
func unescapeCQ(s string) string {
	s = strings.ReplaceAll(s, "&#44;", ",")
	s = strings.ReplaceAll(s, "&#91;", "[")
	s = strings.ReplaceAll(s, "&#93;", "]")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}
