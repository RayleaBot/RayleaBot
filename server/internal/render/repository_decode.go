package render

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func decodeStoredManifest(templateID string, manifestJSONText string) (templateManifest, error) {
	var manifestJSON map[string]any
	if err := jsonUnmarshalObject([]byte(manifestJSONText), &manifestJSON); err != nil {
		return templateManifest{}, fmt.Errorf("decode stored render template manifest for %s: %w", templateID, err)
	}

	manifest, _, err := parseTemplateManifest(templateID, manifestJSON)
	if err != nil {
		return templateManifest{}, fmt.Errorf("decode stored render template manifest for %s: %w", templateID, err)
	}
	return manifest, nil
}

func decodeStoredSource(templateID string, revision storedTemplateRevision) (TemplateSource, error) {
	var manifestJSON map[string]any
	if err := jsonUnmarshalObject([]byte(revision.ManifestJSON), &manifestJSON); err != nil {
		return TemplateSource{}, fmt.Errorf("decode stored render template manifest for %s/%s: %w", templateID, revision.RevisionID, err)
	}

	var inputSchemaJSON map[string]any
	if revision.InputSchemaJSON.Valid && revision.InputSchemaJSON.String != "" {
		if err := jsonUnmarshalObject([]byte(revision.InputSchemaJSON.String), &inputSchemaJSON); err != nil {
			return TemplateSource{}, fmt.Errorf("decode stored render input schema for %s/%s: %w", templateID, revision.RevisionID, err)
		}
	}

	return TemplateSource{
		ManifestJSON:    manifestJSON,
		HTML:            revision.HTML,
		Stylesheet:      revision.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}, nil
}

func jsonUnmarshalObject(encoded []byte, target *map[string]any) error {
	if len(encoded) == 0 {
		*target = nil
		return nil
	}
	return json.Unmarshal(encoded, target)
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func pointerStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func normalizedTemplateSourceInfo(source TemplateSourceInfo) TemplateSourceInfo {
	source.Type = strings.TrimSpace(source.Type)
	source.PluginID = strings.TrimSpace(source.PluginID)
	source.LocalID = strings.TrimSpace(source.LocalID)
	if source.Type == "" {
		source.Type = "system"
	}
	if source.Type != "plugin" {
		return TemplateSourceInfo{Type: "system"}
	}
	return source
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
