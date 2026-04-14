package logging

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
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
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, textsafe.SanitizeString(item))
		}
		return items
	case string:
		return textsafe.SanitizeString(typed)
	default:
		return typed
	}
}

func normalizeProtocolDetails(protocol string, details map[string]any) map[string]any {
	normalized := sanitizeDetailsMap(cloneDetailsMap(details))
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
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

	mergeDetailField(sender, "user_id", details["sender_id"])
	mergeDetailField(sender, "user_id", details["user_id"])
	mergeDetailField(sender, "nickname", details["sender_nickname"])
	mergeDetailField(sender, "card", details["sender_card"])
	mergeDetailField(sender, "role", details["sender_role"])
	mergeDetailField(sender, "title", details["sender_title"])

	if len(sender) > 0 {
		details["sender"] = sender
		delete(details, "sender_id")
		delete(details, "sender_nickname")
		delete(details, "sender_card")
		delete(details, "sender_role")
		delete(details, "sender_title")

		if detailValuesEqual(details["user_id"], sender["user_id"]) {
			delete(details, "user_id")
		}
	}

	if detailValuesEqual(details["time"], details["event_timestamp"]) {
		delete(details, "time")
	}
	if detailValuesEqual(details["group_id"], details["conversation_id"]) {
		delete(details, "group_id")
	}
	if detailValuesEqual(details["real_id"], details["message_id"]) {
		delete(details, "real_id")
	}
	if detailValuesEqual(details["message_seq"], details["message_id"]) {
		delete(details, "message_seq")
	}

	return details
}

func mergeDetailField(target map[string]any, key string, value any) {
	if target == nil || !hasDetailValue(value) {
		return
	}
	if hasDetailValue(target[key]) {
		return
	}
	target[key] = cloneDetailValue(value)
}

func hasDetailValue(value any) bool {
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

func detailValuesEqual(left, right any) bool {
	normalizedLeft, ok := normalizeComparableDetailValue(left)
	if !ok {
		return false
	}

	normalizedRight, ok := normalizeComparableDetailValue(right)
	if !ok {
		return false
	}

	return normalizedLeft == normalizedRight
}

func normalizeComparableDetailValue(value any) (string, bool) {
	if numeric, ok := detailNumber(value); ok {
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

func detailNumber(value any) (float64, bool) {
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
