package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func parseTemplateManifest(expectedTemplateID string, manifestJSON map[string]any) (Manifest, map[string]any, error) {
	if manifestJSON == nil {
		return Manifest{}, nil, fmt.Errorf("manifest_json must be an object")
	}

	id, err := readRequiredString(manifestJSON, "id")
	if err != nil {
		return Manifest{}, nil, err
	}
	if !templateIDPattern.MatchString(id) {
		return Manifest{}, nil, fmt.Errorf("manifest_json.id contains unsupported characters")
	}
	if expectedTemplateID != "" && id != expectedTemplateID {
		return Manifest{}, nil, fmt.Errorf("manifest id %q does not match template path %q", id, expectedTemplateID)
	}

	version, err := readOptionalString(manifestJSON, "version", defaultTemplateVersion)
	if err != nil {
		return Manifest{}, nil, err
	}
	entryHTML, err := readOptionalString(manifestJSON, "entry_html", defaultTemplateHTMLFile)
	if err != nil {
		return Manifest{}, nil, err
	}
	stylesheet, err := readOptionalString(manifestJSON, "stylesheet", defaultTemplateStylesheetFile)
	if err != nil {
		return Manifest{}, nil, err
	}
	inputSchema, err := readOptionalNullableString(manifestJSON, "input_schema")
	if err != nil {
		return Manifest{}, nil, err
	}
	width, err := readOptionalInt(manifestJSON, "width", defaultTemplateWidth)
	if err != nil {
		return Manifest{}, nil, err
	}
	height, err := readOptionalInt(manifestJSON, "height", defaultTemplateHeight)
	if err != nil {
		return Manifest{}, nil, err
	}

	if inputSchema != nil && strings.TrimSpace(*inputSchema) == "" {
		inputSchema = nil
	}

	manifest := Manifest{
		ID:          id,
		Version:     version,
		EntryHTML:   entryHTML,
		Stylesheet:  stylesheet,
		InputSchema: inputSchema,
		Width:       width,
		Height:      height,
	}

	return manifest, manifestToJSON(manifest), nil
}

func manifestToJSON(manifest Manifest) map[string]any {
	document := map[string]any{
		"id":         manifest.ID,
		"version":    manifest.Version,
		"entry_html": manifest.EntryHTML,
		"stylesheet": manifest.Stylesheet,
		"width":      manifest.Width,
		"height":     manifest.Height,
	}
	if manifest.InputSchema != nil {
		document["input_schema"] = *manifest.InputSchema
	}
	return document
}

func SortedIDs(seeds map[string]Seed) []string {
	ids := make([]string, 0, len(seeds))
	for templateID := range seeds {
		ids = append(ids, templateID)
	}
	sort.Strings(ids)
	return ids
}

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
