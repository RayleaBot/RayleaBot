package segments

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

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
			text := textsafe.SanitizeString(unescapeCQ(remaining))
			if text != "" {
				segments = append(segments, MessageSegment{
					Type: "text",
					Data: map[string]any{"text": text},
				})
			}
			break
		}

		if idx > 0 {
			text := textsafe.SanitizeString(unescapeCQ(remaining[:idx]))
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
			text := textsafe.SanitizeString(unescapeCQ(remaining))
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

// ParseCQString parses a OneBot11 CQ-coded message string.
func ParseCQString(raw string) []MessageSegment {
	return parseCQString(raw)
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
		value := textsafe.SanitizeString(unescapeCQ(strings.TrimSpace(kv[1])))
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
