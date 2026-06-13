package logging

import (
	"encoding/json"
	"strings"
)

func encodeDetailsJSON(details map[string]any) (string, error) {
	normalized := sanitizeDetailsMap(cloneDetailsMap(details))
	if len(normalized) == 0 {
		return "{}", nil
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeDetailsJSON(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}

	var details map[string]any
	if err := json.Unmarshal([]byte(raw), &details); err != nil {
		return nil, err
	}
	return sanitizeDetailsMap(details), nil
}

func extractSummaryDetails(body map[string]any) map[string]any {
	if len(body) == 0 {
		return map[string]any{}
	}

	details := make(map[string]any, len(body))
	for key, value := range body {
		switch key {
		case "ts", "level", "component", "msg", "plugin_id", "request_id", "protocol", "log_id":
			continue
		default:
			details[key] = cloneDetailValue(value)
		}
	}
	return sanitizeDetailsMap(details)
}
