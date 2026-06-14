package templates

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func readRequiredString(document map[string]any, key string) (string, error) {
	value, ok := document[key]
	if !ok {
		return "", fmt.Errorf("manifest_json.%s is required", key)
	}

	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("manifest_json.%s must be a string", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("manifest_json.%s is required", key)
	}
	return text, nil
}

func readOptionalString(document map[string]any, key, fallback string) (string, error) {
	value, ok := document[key]
	if !ok || value == nil {
		return fallback, nil
	}

	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("manifest_json.%s must be a string", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fallback, nil
	}
	return text, nil
}

func readOptionalNullableString(document map[string]any, key string) (*string, error) {
	value, ok := document[key]
	if !ok || value == nil {
		return nil, nil
	}

	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("manifest_json.%s must be a string or null", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	return &text, nil
}

func readOptionalInt(document map[string]any, key string, fallback int) (int, error) {
	value, ok := document[key]
	if !ok || value == nil {
		return fallback, nil
	}

	switch typed := value.(type) {
	case float64:
		if typed <= 0 || typed != float64(int(typed)) {
			return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
		}
		return int(typed), nil
	case int:
		if typed <= 0 {
			return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
		}
		return typed, nil
	case int32:
		if typed <= 0 {
			return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
		}
		return int(typed), nil
	case int64:
		if typed <= 0 {
			return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
		}
		return int(typed), nil
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("manifest_json.%s must be a positive integer", key)
	}
}
