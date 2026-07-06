package logging

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/redact"
)

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

func EncodeJSON(details map[string]any) (string, error) {
	normalized := sanitizeMap(CloneMap(details))
	if len(normalized) == 0 {
		return "{}", nil
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func DecodeJSON(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}

	var details map[string]any
	if err := json.Unmarshal([]byte(raw), &details); err != nil {
		return nil, err
	}
	return sanitizeMap(details), nil
}

func NormalizeProtocol(protocol string, details map[string]any) map[string]any {
	normalized := sanitizeMap(CloneMap(details))
	switch strings.TrimSpace(protocol) {
	case "onebot11":
		return compactOneBot11LogDetails(normalized)
	default:
		return normalized
	}
}

func compactOneBot11LogDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sender, _ := details["sender"].(map[string]any)
	if sender == nil {
		sender = map[string]any{}
	}

	mergeField(sender, "user_id", details["sender_id"])
	mergeField(sender, "user_id", details["user_id"])
	mergeField(sender, "nickname", details["sender_nickname"])
	mergeField(sender, "card", details["sender_card"])
	mergeField(sender, "role", details["sender_role"])
	mergeField(sender, "title", details["sender_title"])

	if len(sender) > 0 {
		details["sender"] = sender
		delete(details, "sender_id")
		delete(details, "sender_nickname")
		delete(details, "sender_card")
		delete(details, "sender_role")
		delete(details, "sender_title")

		if valuesEqual(details["user_id"], sender["user_id"]) {
			delete(details, "user_id")
		}
	}

	if valuesEqual(details["time"], details["event_timestamp"]) {
		delete(details, "time")
	}
	if valuesEqual(details["group_id"], details["conversation_id"]) {
		delete(details, "group_id")
	}
	if valuesEqual(details["real_id"], details["message_id"]) {
		delete(details, "real_id")
	}
	if valuesEqual(details["message_seq"], details["message_id"]) {
		delete(details, "message_seq")
	}

	return details
}

func ExtractSummary(body map[string]any) map[string]any {
	if len(body) == 0 {
		return map[string]any{}
	}

	details := make(map[string]any, len(body))
	for key, value := range body {
		switch key {
		case "ts", "level", "component", "msg", "plugin_id", "request_id", "protocol", "log_id":
			continue
		default:
			details[key] = cloneValue(value)
		}
	}
	return sanitizeMap(details)
}

func sanitizeMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sanitized := make(map[string]any, len(details))
	for key, value := range details {
		if isSensitiveKey(key) {
			continue
		}
		sanitized[key] = sanitizeValue(value)
	}
	return sanitized
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeValue(item))
		}
		return items
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, redact.SanitizeString(item))
		}
		return items
	case string:
		return redact.SanitizeString(typed)
	default:
		return typed
	}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}

	for _, marker := range []string{"access_token", "authorization", "cookie", "proxy_url", "secret", "token"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func mergeField(target map[string]any, key string, value any) {
	if target == nil || !hasValue(value) {
		return
	}
	if hasValue(target[key]) {
		return
	}
	target[key] = cloneValue(value)
}

func hasValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

func valuesEqual(left, right any) bool {
	normalizedLeft, ok := normalizeComparableValue(left)
	if !ok {
		return false
	}

	normalizedRight, ok := normalizeComparableValue(right)
	if !ok {
		return false
	}

	return normalizedLeft == normalizedRight
}

func normalizeComparableValue(value any) (string, bool) {
	if numeric, ok := number(value); ok {
		return "n:" + strconv.FormatFloat(numeric, 'f', -1, 64), true
	}

	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return "s:" + trimmed, true
	default:
		trimmed := strings.TrimSpace(fmt.Sprint(typed))
		if trimmed == "" {
			return "", false
		}
		return "s:" + trimmed, true
	}
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case float32:
		number := float64(typed)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return number, true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}
