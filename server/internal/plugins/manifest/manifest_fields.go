package pluginmanifest

func extractStringField(document map[string]any, key string) (string, bool) {
	value, ok := document[key]
	if !ok {
		return "", false
	}

	stringValue, ok := value.(string)
	if !ok || stringValue == "" {
		return "", false
	}

	return stringValue, true
}

func stringField(document map[string]any, key string) string {
	value, ok := document[key]
	if !ok {
		return ""
	}

	stringValue, ok := value.(string)
	if !ok {
		return ""
	}

	return stringValue
}

func manifestBoolField(document map[string]any, key string) bool {
	value, ok := document[key]
	if !ok {
		return false
	}
	booleanValue, ok := value.(bool)
	if !ok {
		return false
	}
	return booleanValue
}

func manifestConcurrency(document map[string]any) int {
	value, ok := document["concurrency"]
	if !ok {
		return 1
	}
	switch typed := value.(type) {
	case int:
		if typed >= 1 {
			return typed
		}
	case int64:
		if typed >= 1 {
			return int(typed)
		}
	case float64:
		if typed >= 1 {
			return int(typed)
		}
	}
	return 1
}

func manifestRole(document map[string]any, sourceRoot string) string {
	role := stringField(document, "role")
	if role != "" {
		return role
	}

	switch sourceRoot {
	case "plugins/builtin":
		return "builtin"
	case "examples/plugins":
		return "example"
	case "plugins/dev":
		return "dev"
	default:
		return "user"
	}
}

func manifestObjectField(document map[string]any, key string) map[string]any {
	value, ok := document[key].(map[string]any)
	if !ok {
		return nil
	}
	return cloneMap(value)
}

func defaultDesiredStateForSourceRoot(sourceRoot string) string {
	if sourceRoot == "plugins/builtin" {
		return DesiredStateEnabled
	}
	return DesiredStateDisabled
}

func stringListField(document map[string]any, key string) []string {
	values, ok := document[key].([]any)
	if !ok {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		items = append(items, text)
	}
	return items
}
