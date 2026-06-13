package render

import (
	"html/template"
	"regexp"

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
	PluginID     string
	LocalID      string
	Dir          string
	ResourceRoot string
}
