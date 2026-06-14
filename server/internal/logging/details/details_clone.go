package details

func CloneMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneValue(item))
		}
		return items
	default:
		return typed
	}
}
