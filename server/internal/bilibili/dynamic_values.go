package bilibili

import (
	"strconv"
	"strings"
)

func nested(values map[string]any, path ...string) any {
	var current any = values
	for _, key := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapped[key]
	}
	return current
}
func nestedMap(values map[string]any, path ...string) map[string]any {
	return mapFromAny(nested(values, path...))
}
func nestedList(values map[string]any, path ...string) []any {
	switch typed := nested(values, path...).(type) {
	case []any:
		return typed
	default:
		return nil
	}
}
func listFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items
	default:
		return nil
	}
}
func mapFromAny(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{}
}
func intValue(value any) int {
	return int(int64Value(value))
}
func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		number, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return number
	default:
		return 0
	}
}
