package localaction

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func requiredActionString(data map[string]any, key string) (string, error) {
	if len(data) == 0 {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	value, ok := data[key]
	if !ok {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	return text, nil
}

func optionalActionString(data map[string]any, key string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", false
	}
	return text, true
}

func normalizeNumericValue(value any) any {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	default:
		return value
	}
}

func oneBotAPIValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return raw
		}
	}
	return raw
}
