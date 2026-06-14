package protocol

func payloadString(values map[string]any, key string) (string, bool) {
	value, ok := values[key].(string)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func payloadInt64(values map[string]any, key string) (int64, bool) {
	switch value := values[key].(type) {
	case int64:
		if value <= 0 {
			return 0, false
		}
		return value, true
	case int:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	case float64:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}

func payloadInt(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		if value <= 0 {
			return 0, false
		}
		return value, true
	case int64:
		if value <= 0 {
			return 0, false
		}
		return int(value), true
	case float64:
		if value <= 0 {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func payloadIntAllowZero(values map[string]any, key string) (int, bool) {
	switch value := values[key].(type) {
	case int:
		if value < 0 {
			return 0, false
		}
		return value, true
	case int64:
		if value < 0 {
			return 0, false
		}
		return int(value), true
	case float64:
		if value < 0 {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func payloadMap(values map[string]any, key string) (map[string]any, bool) {
	raw, ok := values[key].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	cloned := make(map[string]any, len(raw))
	for mapKey, value := range raw {
		cloned[mapKey] = value
	}
	return cloned, true
}
