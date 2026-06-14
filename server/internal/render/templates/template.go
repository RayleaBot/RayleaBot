package templates

import (
	"html/template"
	"regexp"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

const (
	ManifestFilename              = "template.json"
	DefaultPreviewData            = "preview.json"
	defaultTemplateVersion        = "1"
	defaultTemplateHTMLFile       = "template.html"
	defaultTemplateStylesheetFile = "styles.css"
	defaultTemplateInputSchema    = "input.schema.json"
	defaultTemplateWidth          = 960
	defaultTemplateHeight         = 640
)

var templateIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type TemplateSource = renderrepo.TemplateSource
type TemplateFiles = renderrepo.TemplateFiles

type Root struct {
	TemplateDir  string
	ResourceRoot string
}

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type TemplateDraft struct {
	Source TemplateSource `json:"source"`
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

type Manifest struct {
	ID          string
	Version     string
	EntryHTML   string
	Stylesheet  string
	InputSchema *string
	Width       int
	Height      int
}

type SourceBundle struct {
	Manifest           Manifest
	NormalizedManifest map[string]any
	Source             TemplateSource
	Files              TemplateFiles
	Digest             string
}

type CompiledTemplate struct {
	Bundle     SourceBundle
	Stylesheet template.CSS
	Schema     *schema.Validator
	HTML       *template.Template
}

type Seed struct {
	Source   TemplateSource
	Compiled *CompiledTemplate
}
