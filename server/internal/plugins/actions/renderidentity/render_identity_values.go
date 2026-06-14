package renderidentity

import (
	"fmt"
	"strings"
)

func CloneData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data)+3)
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func objectValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstText(values ...any) string {
	for _, value := range values {
		text := textValue(value)
		if text != "" {
			return text
		}
	}
	return ""
}

func textValue(value any) string {
	if value == nil {
		return ""
	}
	if _, ok := value.(bool); ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
