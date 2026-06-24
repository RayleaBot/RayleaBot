package repository

const templateManifestFilename = "template.json"

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

type templateManifest struct {
	ID          string
	Version     string
	EntryHTML   string
	Stylesheet  string
	InputSchema *string
	Width       int
	Height      int
}
