package app

type renderTemplateSummary struct {
	ID             string               `json:"id"`
	Version        string               `json:"version"`
	Width          int                  `json:"width"`
	Height         int                  `json:"height"`
	HasInputSchema bool                 `json:"has_input_schema"`
	UpdatedAt      string               `json:"updated_at"`
	Source         renderTemplateSource `json:"source"`
}

type renderTemplateDetail struct {
	ID              string               `json:"id"`
	Version         string               `json:"version"`
	Width           int                  `json:"width"`
	Height          int                  `json:"height"`
	HasInputSchema  bool                 `json:"has_input_schema"`
	UpdatedAt       string               `json:"updated_at"`
	Source          renderTemplateSource `json:"source"`
	InputSchemaJSON map[string]any       `json:"input_schema_json"`
	PreviewDataJSON map[string]any       `json:"preview_data_json"`
}

type renderTemplateSource struct {
	Type     string  `json:"type"`
	PluginID *string `json:"plugin_id"`
	LocalID  *string `json:"local_id"`
}

type renderTemplateListResponse struct {
	Items []renderTemplateSummary `json:"items"`
}

type renderTemplateDetailResponse struct {
	Template renderTemplateDetail `json:"template"`
}

type renderTemplatePreviewHTMLRequest struct {
	Theme string         `json:"theme,omitempty"`
	Data  map[string]any `json:"data"`
}

type renderTemplatePreviewHTMLResponse struct {
	TemplateID string `json:"template_id"`
	RevisionID string `json:"revision_id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HTML       string `json:"html"`
}
