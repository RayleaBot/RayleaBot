package details

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func sanitizeMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sanitized := make(map[string]any, len(details))
	for key, value := range details {
		if isSensitiveKey(key) {
			continue
		}
		sanitized[key] = sanitizeValue(value)
	}
	return sanitized
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeValue(item))
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

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}

	for _, marker := range []string{"access_token", "authorization", "cookie", "proxy_url", "secret", "token"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
