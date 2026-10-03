package config

import (
	"math"
	"unicode/utf8"
)

// CanonicalJSONDocumentFromTyped returns an owned document with the same value
// types as CloneDocument(CanonicalDocumentFromTyped(cfg)), without serializing it.
func CanonicalJSONDocumentFromTyped(cfg Config) map[string]any {
	document := canonicalDocumentFromTyped(cfg)
	if _, ok := normalizeOwnedConfigJSON(document); !ok {
		return CloneDocument(document)
	}
	return document
}

// Maps and []any come from the canonical builder. []string may still belong to
// cfg, so converting those slices also isolates the result from the input.
func normalizeOwnedConfigJSON(value any) (any, bool) {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			normalized, ok := normalizeOwnedConfigJSON(item)
			if !ok {
				return nil, false
			}
			value[key] = normalized
		}
		return value, true
	case []any:
		for index, item := range value {
			normalized, ok := normalizeOwnedConfigJSON(item)
			if !ok {
				return nil, false
			}
			value[index] = normalized
		}
		return value, true
	case []string:
		if value == nil {
			return nil, true
		}
		items := make([]any, len(value))
		for index, item := range value {
			items[index] = configJSONString(item)
		}
		return items, true
	case string:
		return configJSONString(value), true
	case int:
		return float64(value), true
	case float64:
		return value, !math.IsNaN(value) && !math.IsInf(value, 0)
	case bool, nil:
		return value, true
	default:
		// Keep JSON handling authoritative if the canonical builder gains other types.
		return nil, false
	}
}

func configJSONString(value string) string {
	if utf8.ValidString(value) {
		return value
	}
	return string([]rune(value))
}
