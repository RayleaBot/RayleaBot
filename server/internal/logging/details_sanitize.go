package logging

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func sanitizeDetailsMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sanitized := make(map[string]any, len(details))
	for key, value := range details {
		if isSensitiveDetailKey(key) {
			continue
		}
		sanitized[key] = sanitizeDetailValue(value)
	}
	return sanitized
}

func sanitizeDetailValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeDetailsMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeDetailValue(item))
		}
		return items
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, textsafe.SanitizeString(item))
		}
		return items
	case string:
		return textsafe.SanitizeString(typed)
	default:
		return typed
	}
}

func isSensitiveDetailKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}

	for _, marker := range []string{"access_token", "authorization", "cookie", "secret", "token"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
