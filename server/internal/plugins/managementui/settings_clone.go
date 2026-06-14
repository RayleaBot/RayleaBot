package managementui

func cloneSettingsMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneSettingsValue(value)
	}
	return cloned
}

func cloneSettingsSlice(values []any) []any {
	if len(values) == 0 {
		return []any{}
	}

	cloned := make([]any, len(values))
	for index, value := range values {
		cloned[index] = cloneSettingsValue(value)
	}
	return cloned
}

func cloneSettingsValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSettingsMap(typed)
	case []any:
		return cloneSettingsSlice(typed)
	default:
		return typed
	}
}

func ensureSettingsMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}
