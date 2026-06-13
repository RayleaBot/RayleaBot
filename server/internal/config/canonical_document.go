package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

func canonicalizeDocument(raw map[string]any) (map[string]any, error) {
	normalized, err := normalizeDocument(raw)
	if err != nil {
		return nil, err
	}

	document, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("normalized document is not an object")
	}
	document = stripNullValues(document)

	cloned := CloneDocument(document)
	if cloned == nil {
		cloned = map[string]any{}
	}
	if version := strings.TrimSpace(stringValue(cloned["schema_version"])); version == "" {
		cloned["schema_version"] = currentSchemaVersion
	}
	normalizeOneBotSection(cloned)
	return cloned, nil
}

func stripNullValues(document map[string]any) map[string]any {
	if document == nil {
		return nil
	}

	cleaned := make(map[string]any, len(document))
	for key, value := range document {
		cleanedValue, keep := stripNullValue(value)
		if !keep {
			continue
		}
		cleaned[key] = cleanedValue
	}
	return cleaned
}

func stripNullValue(value any) (any, bool) {
	if value == nil {
		return nil, false
	}

	switch typed := value.(type) {
	case map[string]any:
		return stripNullValues(typed), true
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			cleanedItem, keep := stripNullValue(item)
			if !keep {
				continue
			}
			items = append(items, cleanedItem)
		}
		return items, true
	default:
		return value, true
	}
}

func section(document map[string]any, key string) map[string]any {
	value, ok := document[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func transportSection(document map[string]any, key string) map[string]any {
	value, ok := document[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func mergeDocuments(base, overlay map[string]any) map[string]any {
	result := CloneDocument(base)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range overlay {
		targetSection, targetIsMap := result[key].(map[string]any)
		sourceSection, sourceIsMap := value.(map[string]any)
		if targetIsMap && sourceIsMap {
			result[key] = mergeDocuments(targetSection, sourceSection)
			continue
		}
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(bytes, &cloned); err != nil {
		return value
	}
	return cloned
}

func decodeTypedConfig(document map[string]any) (Config, error) {
	var cfg Config
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
