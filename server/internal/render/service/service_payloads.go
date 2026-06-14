package service

type Request struct {
	Template string         `json:"template"`
	Theme    string         `json:"theme,omitempty"`
	Output   string         `json:"output,omitempty"`
	Data     map[string]any `json:"data"`
	Plugin   *PluginContext `json:"-"`
}

type PluginContext struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type PreviewHTML struct {
	TemplateID string
	RevisionID string
	Width      int
	Height     int
	HTML       string
}

type TemplateAsset struct {
	Path string
}
