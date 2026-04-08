package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var logIDSequence atomic.Uint64

func generateLogID() string {
	return fmt.Sprintf("log_%d_%06d", time.Now().UTC().UnixNano(), logIDSequence.Add(1))
}

func cloneDetailsMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = cloneDetailValue(value)
	}
	return cloned
}

func cloneDetailValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneDetailsMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneDetailValue(item))
		}
		return items
	default:
		return typed
	}
}

func sanitizeDetailsMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sanitized := make(map[string]any, len(details))
	for key, value := range details {
		if isSensitiveDetailKey(key) {
			continue
		}
		sanitized[key] = sanitizeDetailValue(value)
	}
	return sanitized
}

func sanitizeDetailValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeDetailsMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeDetailValue(item))
		}
		return items
	default:
		return typed
	}
}

func isSensitiveDetailKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}

	for _, marker := range []string{"access_token", "authorization", "cookie", "secret", "token"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

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
