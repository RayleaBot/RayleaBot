package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

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
			seg.Data[normalizeCQKey(item.Type, k)] = textsafe.SanitizeAny(v)
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

// anyToString converts a JSON-decoded value to a string representation.
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return textsafe.SanitizeString(val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case json.Number:
		return val.String()
	default:
		b, _ := json.Marshal(v)
		return textsafe.SanitizeString(string(b))
	}
}
