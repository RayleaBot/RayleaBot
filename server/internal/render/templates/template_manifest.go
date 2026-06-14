package templates

import (
	"fmt"
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
