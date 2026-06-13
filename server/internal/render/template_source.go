package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func buildTemplateSourceBundle(expectedTemplateID string, source TemplateSource) (templateSourceBundle, error) {
	manifest, normalizedManifest, err := parseTemplateManifest(expectedTemplateID, source.ManifestJSON)
	if err != nil {
		return templateSourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     err,
		}
	}

	inputSchemaJSON, err := normalizeOptionalJSONObject(source.InputSchemaJSON, "input_schema_json")
	if err != nil {
		return templateSourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     err,
		}
	}

	if manifest.InputSchema == nil && inputSchemaJSON != nil {
		defaultInputSchema := defaultTemplateInputSchema
		manifest.InputSchema = &defaultInputSchema
		normalizedManifest["input_schema"] = defaultInputSchema
	}
	if manifest.InputSchema != nil && inputSchemaJSON == nil {
		return templateSourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     fmt.Errorf("manifest declares input_schema but input_schema_json is null"),
		}
	}

	normalizedSource := TemplateSource{
		ManifestJSON:    normalizedManifest,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}

	return templateSourceBundle{
		manifest:           manifest,
		normalizedManifest: normalizedManifest,
		source:             normalizedSource,
		files: TemplateFiles{
			Manifest:    templateManifestFilename,
			HTML:        manifest.EntryHTML,
			Stylesheet:  manifest.Stylesheet,
			InputSchema: manifest.InputSchema,
		},
		digest: digestTemplateSource(normalizedSource),
	}, nil
}

func normalizeOptionalJSONObject(raw map[string]any, field string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not serializable: %w", field, err)
	}

	var normalized map[string]any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", field, err)
	}

	return normalized, nil
}

func digestTemplateSource(source TemplateSource) string {
	payload := struct {
		ManifestJSON    map[string]any `json:"manifest_json"`
		HTML            string         `json:"html"`
		Stylesheet      string         `json:"stylesheet"`
		InputSchemaJSON map[string]any `json:"input_schema_json"`
	}{
		ManifestJSON:    source.ManifestJSON,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: source.InputSchemaJSON,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
