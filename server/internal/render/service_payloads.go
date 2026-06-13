package render

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

type Document struct {
	Template          string
	Theme             string
	Output            string
	BaseURL           string
	Width             int
	Height            int
	AutoHeight        bool
	DeviceScaleFactor float64
	HTML              string
}

type Result struct {
	ArtifactID string
	ImagePath  string
	MIME       string
	CacheKey   string
	Template   string
	Theme      string
	FromCache  bool
}

type PreviewHTML struct {
	TemplateID string
	RevisionID string
	Width      int
	Height     int
	HTML       string
}

type Artifact struct {
	ArtifactID string
	MIME       string
	Path       string
}

type TemplateAsset struct {
	Path string
}
