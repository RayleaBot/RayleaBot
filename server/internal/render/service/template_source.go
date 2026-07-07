package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

func BuildSourceBundle(expectedTemplateID string, source renderrepo.TemplateSource) (SourceBundle, error) {
	manifest, normalizedManifest, err := parseTemplateManifest(expectedTemplateID, source.ManifestJSON)
	if err != nil {
		return SourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     err,
		}
	}

	inputSchemaJSON, err := normalizeOptionalJSONObject(source.InputSchemaJSON, "input_schema_json")
	if err != nil {
		return SourceBundle{}, &Error{
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
		return SourceBundle{}, &Error{
			Code:    "platform.template_source_invalid",
			Message: "render template source is invalid",
			Err:     fmt.Errorf("manifest declares input_schema but input_schema_json is null"),
		}
	}

	normalizedSource := renderrepo.TemplateSource{
		ManifestJSON:    normalizedManifest,
		HTML:            source.HTML,
		Stylesheet:      source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}

	return SourceBundle{
		Manifest:           manifest,
		NormalizedManifest: normalizedManifest,
		Source:             normalizedSource,
		Files: renderrepo.TemplateFiles{
			Manifest:    ManifestFilename,
			HTML:        manifest.EntryHTML,
			Stylesheet:  manifest.Stylesheet,
			InputSchema: manifest.InputSchema,
		},
		Digest: DigestSource(normalizedSource),
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

func DigestSource(source renderrepo.TemplateSource) string {
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

func ResourceDigest(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	assetsDir := filepath.Join(templateDir, "assets")
	if !pathWithinRoot(templateDir, assetsDir) {
		return ""
	}

	digest := sha256.New()
	walkErr := filepath.WalkDir(assetsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest.Write(content)
		digest.Write([]byte{0})
		return nil
	})
	if walkErr != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))
}
