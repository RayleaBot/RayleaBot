package onebot

import (
	"fmt"
	"strings"
)

func normalizeParams(raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	params := make(map[string]any, len(raw))
	for key, value := range raw {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		switch normalizedKey {
		case "conversation_id":
			continue
		case "limit":
			params[normalizedKey] = normalizeNumericValue(value)
		case "duration_seconds":
			params["duration"] = normalizeNumericValue(value)
		case "emoji":
			params["emoji_id"] = value
		case "target_id", "user_id", "group_id", "message_id":
			params[normalizedKey] = apiValue(fmt.Sprint(value))
		default:
			params[normalizedKey] = value
		}
	}
	return params, nil
}

func defaultResult(collectionKey string, result any) map[string]any {
	switch typed := result.(type) {
	case nil:
		return map[string]any{"ok": true}
	case map[string]any:
		if len(typed) == 0 {
			return map[string]any{"ok": true}
		}
		return typed
	case []any:
		return map[string]any{collectionKeyOrDefault(collectionKey): typed}
	default:
		return map[string]any{"value": typed}
	}
}

func collectionKeyOrDefault(collectionKey string) string {
	if strings.TrimSpace(collectionKey) == "" {
		return "items"
	}
	return collectionKey
}
