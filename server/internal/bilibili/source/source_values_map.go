package source

import "encoding/json"

func intFromMap(target any, key string) int {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func stringFromMap(target any, key string) string {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	return stringValue(values[key])
}
