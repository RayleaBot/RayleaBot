package config

import (
	"encoding/json"
	"fmt"
)

// Filter before JSON conversion so even non-JSON YAML values in an ignored
// section cannot prevent the declared configuration from loading.
func filterConfigDocument(document map[string]any) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(ConfigUserSchemaJSON, &root); err != nil {
		return nil, fmt.Errorf("parse config field schema: %w", err)
	}
	filtered, err := filterConfigValue(document, root, root)
	if err != nil {
		return nil, err
	}
	return filtered.(map[string]any), nil
}

func filterConfigValue(value any, node, root map[string]any) (any, error) {
	if ref, ok := node["$ref"].(string); ok {
		resolved, err := resolveSchemaRef(root, ref)
		if err != nil {
			return nil, err
		}
		return filterConfigValue(value, resolved, root)
	}
	switch typed := value.(type) {
	case map[string]any:
		properties, ok := node["properties"].(map[string]any)
		if !ok {
			return value, nil
		}
		filtered := make(map[string]any, len(typed))
		for name, child := range typed {
			property, known := properties[name].(map[string]any)
			if !known {
				continue
			}
			result, err := filterConfigValue(child, property, root)
			if err != nil {
				return nil, err
			}
			filtered[name] = result
		}
		return filtered, nil
	case []any:
		items, ok := node["items"].(map[string]any)
		if !ok {
			return value, nil
		}
		filtered := make([]any, len(typed))
		for index, child := range typed {
			result, err := filterConfigValue(child, items, root)
			if err != nil {
				return nil, err
			}
			filtered[index] = result
		}
		return filtered, nil
	default:
		return value, nil
	}
}
