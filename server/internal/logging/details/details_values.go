package details

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

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
