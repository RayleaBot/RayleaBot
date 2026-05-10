package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

const (
	templateManifestFilename      = "template.json"
	defaultTemplateVersion        = "1"
	defaultTemplateHTMLFile       = "template.html"
	defaultTemplateStylesheetFile = "styles.css"
	defaultTemplateInputSchema    = "input.schema.json"
	defaultTemplatePreviewData    = "preview.json"
	defaultTemplateWidth          = 960
	defaultTemplateHeight         = 640
)

var templateIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type TemplateDraft struct {
	Source TemplateSource `json:"source"`
}

type TemplateSource struct {
	ManifestJSON    map[string]any `json:"manifest_json"`
	HTML            string         `json:"html"`
	Stylesheet      string         `json:"stylesheet"`
	InputSchemaJSON map[string]any `json:"input_schema_json"`
}

type TemplateFiles struct {
	Manifest    string  `json:"manifest"`
	HTML        string  `json:"html"`
	Stylesheet  string  `json:"stylesheet"`
	InputSchema *string `json:"input_schema"`
}

type TemplateValidationStatus struct {
	Valid      bool   `json:"valid"`
	CheckedAt  string `json:"checked_at"`
	IssueCount int    `json:"issue_count"`
}

type TemplateSourceInfo struct {
	Type     string `json:"type"`
	PluginID string `json:"plugin_id,omitempty"`
	LocalID  string `json:"local_id,omitempty"`
}

type TemplateVersion struct {
	RevisionID      string  `json:"revision_id"`
	TemplateVersion string  `json:"template_version"`
	SavedAt         string  `json:"saved_at"`
	Kind            string  `json:"kind"`
	Message         *string `json:"message"`
}

type TemplateSummary struct {
	ID                string `json:"id"`
	Version           string `json:"version"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	HasInputSchema    bool   `json:"has_input_schema"`
	CurrentRevisionID string `json:"current_revision_id"`
	UpdatedAt         string `json:"updated_at"`
	Source            TemplateSourceInfo
}

type TemplateDetail struct {
	TemplateSummary
	Files           TemplateFiles            `json:"files"`
	CurrentRevision TemplateVersion          `json:"current_revision"`
	LastValidation  TemplateValidationStatus `json:"last_validation"`
}

type TemplateValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type TemplateValidationResult struct {
	Valid              bool                      `json:"valid"`
	Issues             []TemplateValidationIssue `json:"issues"`
	NormalizedManifest map[string]any            `json:"normalized_manifest"`
}

type templateManifest struct {
	ID          string
	Version     string
	EntryHTML   string
	Stylesheet  string
	InputSchema *string
	Width       int
	Height      int
}

type templateSourceBundle struct {
	manifest           templateManifest
	normalizedManifest map[string]any
	source             TemplateSource
	files              TemplateFiles
	digest             string
}

type compiledTemplate struct {
	bundle     templateSourceBundle
	stylesheet template.CSS
	schema     *schema.Validator
	html       *template.Template
}

type templateSeed struct {
	source   TemplateSource
	compiled *compiledTemplate
}

type PluginTemplateSource struct {
	PluginID string
	LocalID  string
	Dir      string
}

func discoverTemplateSeeds(root string, logger *slog.Logger) (map[string]templateSeed, error) {
	if root == "" {
		return map[string]templateSeed{}, nil
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]templateSeed{}, nil
		}
		return nil, fmt.Errorf("inspect templates root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("templates root %s is not a directory", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read templates root %s: %w", root, err)
	}

	seeds := make(map[string]templateSeed, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templateDir := filepath.Join(root, entry.Name())
		seed, err := loadTemplateSeed(templateDir)
		if err != nil {
			if logger != nil {
				logger.Warn(
					"render template skipped",
					"component", "render",
					"template_dir", templateDir,
					"err", err,
				)
			}
			continue
		}
		seeds[seed.compiled.bundle.manifest.ID] = seed
	}

	return seeds, nil
}

func loadTemplateSeed(templateDir string) (templateSeed, error) {
	manifestPath := filepath.Join(templateDir, templateManifestFilename)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return templateSeed{}, fmt.Errorf("read render template manifest %s: %w", manifestPath, err)
	}

	var manifestJSON map[string]any
	if err := json.Unmarshal(manifestBytes, &manifestJSON); err != nil {
		return templateSeed{}, fmt.Errorf("decode render template manifest %s: %w", manifestPath, err)
	}

	manifest, normalizedManifest, err := parseTemplateManifest("", manifestJSON)
	if err != nil {
		return templateSeed{}, fmt.Errorf("load render template manifest %s: %w", manifestPath, err)
	}

	htmlPath, err := templateFilePath(templateDir, manifest.EntryHTML)
	if err != nil {
		return templateSeed{}, fmt.Errorf("resolve render template html for %s: %w", manifest.ID, err)
	}
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return templateSeed{}, fmt.Errorf("read render template html for %s: %w", manifest.ID, err)
	}

	stylesheetPath, err := templateFilePath(templateDir, manifest.Stylesheet)
	if err != nil {
		return templateSeed{}, fmt.Errorf("resolve render template stylesheet for %s: %w", manifest.ID, err)
	}
	stylesheetBytes, err := os.ReadFile(stylesheetPath)
	if err != nil {
		return templateSeed{}, fmt.Errorf("read render template stylesheet for %s: %w", manifest.ID, err)
	}

	var inputSchemaJSON map[string]any
	if manifest.InputSchema != nil {
		inputSchemaPath, err := templateFilePath(templateDir, *manifest.InputSchema)
		if err != nil {
			return templateSeed{}, fmt.Errorf("resolve render input schema for %s: %w", manifest.ID, err)
		}
		inputSchemaBytes, err := os.ReadFile(inputSchemaPath)
		if err != nil {
			return templateSeed{}, fmt.Errorf("read render input schema for %s: %w", manifest.ID, err)
		}
		if err := json.Unmarshal(inputSchemaBytes, &inputSchemaJSON); err != nil {
			return templateSeed{}, fmt.Errorf("decode render input schema for %s: %w", manifest.ID, err)
		}
	}

	source := TemplateSource{
		ManifestJSON:    normalizedManifest,
		HTML:            string(htmlBytes),
		Stylesheet:      string(stylesheetBytes),
		InputSchemaJSON: inputSchemaJSON,
	}

	bundle, err := buildTemplateSourceBundle(manifest.ID, source)
	if err != nil {
		return templateSeed{}, err
	}
	compiled, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		return templateSeed{}, err
	}
	if len(issues) > 0 {
		return templateSeed{}, fmt.Errorf("render template %s is invalid: %s", manifest.ID, issues[0].Message)
	}

	return templateSeed{
		source:   source,
		compiled: compiled,
	}, nil
}

func templateFilePath(templateDir, relativePath string) (string, error) {
	templateDir = strings.TrimSpace(templateDir)
	relativePath = strings.TrimSpace(relativePath)
	if templateDir == "" || relativePath == "" || filepath.IsAbs(filepath.FromSlash(relativePath)) {
		return "", fmt.Errorf("template file path %q is invalid", relativePath)
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("template file path %q is outside template directory", relativePath)
	}

	absoluteRoot, err := filepath.Abs(templateDir)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	if !pathWithinRoot(absoluteRoot, candidate) {
		return "", fmt.Errorf("template file path %q is outside template directory", relativePath)
	}
	return candidate, nil
}

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

func compileTemplateBundle(bundle templateSourceBundle) (*compiledTemplate, []TemplateValidationIssue, error) {
	funcs := template.FuncMap{
		"toJSON": func(value any) template.JS {
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return template.JS("{}")
			}
			return template.JS(encoded)
		},
	}

	compiledHTML, err := template.New(bundle.manifest.ID).Funcs(funcs).Parse(bundle.source.HTML)
	if err != nil {
		return nil, []TemplateValidationIssue{{
			Code:    "html.compile_failed",
			Message: err.Error(),
			Path:    "html",
		}}, nil
	}

	var validator *schema.Validator
	if bundle.source.InputSchemaJSON != nil {
		validator, err = schema.CompileDocument("render-template://"+bundle.manifest.ID+"/input.schema.json", bundle.source.InputSchemaJSON)
		if err != nil {
			return nil, []TemplateValidationIssue{{
				Code:    "input_schema.compile_failed",
				Message: err.Error(),
				Path:    "input_schema_json",
			}}, nil
		}
	}

	return &compiledTemplate{
		bundle:     bundle,
		stylesheet: template.CSS(bundle.source.Stylesheet),
		schema:     validator,
		html:       compiledHTML,
	}, nil, nil
}

func (t *compiledTemplate) renderHTML(theme string, data map[string]any) (string, error) {
	if t == nil {
		return "", fmt.Errorf("render template is not available")
	}

	normalized, err := normalizeTemplateData(data)
	if err != nil {
		return "", &Error{
			Code:    "platform.invalid_request",
			Message: "render input is not serializable",
			Err:     err,
		}
	}

	if t.schema != nil {
		if err := t.schema.Validate(normalized); err != nil {
			return "", &Error{
				Code:    "platform.invalid_request",
				Message: "render input does not match the template schema",
				Err:     err,
			}
		}
	}

	payload := make(map[string]any, len(normalized)+4)
	for key, value := range normalized {
		payload[key] = value
	}
	payload["Theme"] = theme
	payload["theme"] = theme
	payload["Stylesheet"] = t.stylesheet
	payload["stylesheet"] = t.stylesheet

	buffer := &bytes.Buffer{}
	if err := t.html.Execute(buffer, payload); err != nil {
		return "", fmt.Errorf("execute render template %s: %w", t.bundle.manifest.ID, err)
	}

	return buffer.String(), nil
}

func normalizeTemplateData(data map[string]any) (map[string]any, error) {
	if data == nil {
		return map[string]any{}, nil
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var normalized map[string]any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}

func parseTemplateManifest(expectedTemplateID string, manifestJSON map[string]any) (templateManifest, map[string]any, error) {
	if manifestJSON == nil {
		return templateManifest{}, nil, fmt.Errorf("manifest_json must be an object")
	}

	id, err := readRequiredString(manifestJSON, "id")
	if err != nil {
		return templateManifest{}, nil, err
	}
	if !templateIDPattern.MatchString(id) {
		return templateManifest{}, nil, fmt.Errorf("manifest_json.id contains unsupported characters")
	}
	if expectedTemplateID != "" && id != expectedTemplateID {
		return templateManifest{}, nil, fmt.Errorf("manifest id %q does not match template path %q", id, expectedTemplateID)
	}

	version, err := readOptionalString(manifestJSON, "version", defaultTemplateVersion)
	if err != nil {
		return templateManifest{}, nil, err
	}
	entryHTML, err := readOptionalString(manifestJSON, "entry_html", defaultTemplateHTMLFile)
	if err != nil {
		return templateManifest{}, nil, err
	}
	stylesheet, err := readOptionalString(manifestJSON, "stylesheet", defaultTemplateStylesheetFile)
	if err != nil {
		return templateManifest{}, nil, err
	}
	inputSchema, err := readOptionalNullableString(manifestJSON, "input_schema")
	if err != nil {
		return templateManifest{}, nil, err
	}
	width, err := readOptionalInt(manifestJSON, "width", defaultTemplateWidth)
	if err != nil {
		return templateManifest{}, nil, err
	}
	height, err := readOptionalInt(manifestJSON, "height", defaultTemplateHeight)
	if err != nil {
		return templateManifest{}, nil, err
	}

	if inputSchema != nil && strings.TrimSpace(*inputSchema) == "" {
		inputSchema = nil
	}

	manifest := templateManifest{
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

func manifestToJSON(manifest templateManifest) map[string]any {
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

func sortedTemplateIDs(seeds map[string]templateSeed) []string {
	ids := make([]string, 0, len(seeds))
	for templateID := range seeds {
		ids = append(ids, templateID)
	}
	sort.Strings(ids)
	return ids
}
