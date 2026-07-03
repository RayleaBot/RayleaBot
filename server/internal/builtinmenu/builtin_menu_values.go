package menu

import "strings"

func normalizeMenuLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func stringValueFromMap(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}

func stringSliceFromMap(item map[string]any, key string) []string {
	raw, ok := item[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
		return values
	default:
		return nil
	}
}

func firstBuiltinMenuText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
