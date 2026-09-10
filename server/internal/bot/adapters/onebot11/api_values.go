package onebot11

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
)

// extractStringField extracts a string value from a data map.
func extractStringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}

	switch value := data[key].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(redact.SanitizeString(value))
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return redact.SanitizeString(fmt.Sprint(value))
	}
}

func normalizeAPIListWithKeys(value any, keys []string) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case map[string]any:
		for _, key := range keys {
			if items, ok := typed[key].([]any); ok {
				return items, true
			}
			if nested, ok := typed[key].(map[string]any); ok {
				if items, ok := normalizeAPIListWithKeys(nested, keys); ok {
					return items, true
				}
			}
		}
	}
	return nil, false
}

func normalizeAPIResult(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if item == nil {
				result[key] = nil
				continue
			}
			if isIdentifierKey(key) {
				result[key] = extractStringValue(item)
				continue
			}
			result[key] = normalizeAPIResult(item)
		}
		return result
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeAPIResult(item))
		}
		return items
	default:
		return normalizeScalarValue(typed)
	}
}

func normalizeScalarValue(value any) any {
	switch typed := value.(type) {
	case string:
		return redact.SanitizeString(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return value
	}
}

func extractStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(redact.SanitizeString(typed))
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(redact.SanitizeString(fmt.Sprint(typed)))
	}
}

func isIdentifierKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "id" || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_seq")
}
