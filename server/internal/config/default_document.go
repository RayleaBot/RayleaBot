package config

import (
	"encoding/json"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

var defaultDocumentTemplate = mustDefaultDocumentTemplate()

func defaultDocument() map[string]any {
	return CloneDocument(defaultDocumentTemplate)
}

func mustDefaultDocumentTemplate() map[string]any {
	document, err := defaultDocumentFromSchema(schemaassets.ConfigUserSchemaJSON)
	if err != nil {
		panic(fmt.Sprintf("build default config document: %v", err))
	}
	return document
}

func defaultDocumentFromSchema(schemaJSON []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(schemaJSON, &root); err != nil {
		return nil, fmt.Errorf("parse config schema defaults: %w", err)
	}
	value, err := defaultValueFromSchema(root, root)
	if err != nil {
		return nil, err
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config schema default document is %T, want object", value)
	}
	return document, nil
}

func defaultValueFromSchema(root, node map[string]any) (any, error) {
	if ref, ok := node["$ref"].(string); ok {
		resolved, err := resolveSchemaRef(root, ref)
		if err != nil {
			return nil, err
		}
		merged := CloneDocument(resolved)
		for key, value := range node {
			if key == "$ref" {
				continue
			}
			merged[key] = cloneValue(value)
		}
		node = merged
	}

	if value, ok := node["default"]; ok {
		return cloneValue(value), nil
	}
	if value, ok := node["const"]; ok {
		return cloneValue(value), nil
	}

	if node["type"] == "object" {
		return defaultObjectFromSchema(root, node)
	}

	return nil, fmt.Errorf("schema node %q has no default", schemaDescription(node))
}

func defaultObjectFromSchema(root, node map[string]any) (map[string]any, error) {
	rawProperties, _ := node["properties"].(map[string]any)
	rawRequired, _ := node["required"].([]any)
	document := make(map[string]any, len(rawRequired))
	for _, rawName := range rawRequired {
		name, ok := rawName.(string)
		if !ok {
			return nil, fmt.Errorf("schema required entry is %T, want string", rawName)
		}
		rawProperty, ok := rawProperties[name]
		if !ok {
			return nil, fmt.Errorf("required config field %s has no schema property", name)
		}
		property, ok := rawProperty.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema property %s is %T, want object", name, rawProperty)
		}
		value, err := defaultValueFromSchema(root, property)
		if err != nil {
			return nil, fmt.Errorf("default for %s: %w", name, err)
		}
		document[name] = value
	}
	return document, nil
}

func resolveSchemaRef(root map[string]any, ref string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return nil, fmt.Errorf("unsupported schema ref %q", ref)
	}
	defs, _ := root["$defs"].(map[string]any)
	raw, ok := defs[ref[len(prefix):]]
	if !ok {
		return nil, fmt.Errorf("schema ref %q not found", ref)
	}
	resolved, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema ref %q is %T, want object", ref, raw)
	}
	return resolved, nil
}

func schemaDescription(node map[string]any) string {
	if description, ok := node["description"].(string); ok && description != "" {
		return description
	}
	if title, ok := node["title"].(string); ok && title != "" {
		return title
	}
	return "unnamed"
}
