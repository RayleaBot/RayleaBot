package view

type PermissionResponse struct {
	Capability  string  `json:"capability"`
	Requirement string  `json:"requirement"`
	Status      string  `json:"status"`
	Source      string  `json:"source"`
	ExpiresAt   *string `json:"expires_at"`
}

type DependenciesResponse struct {
	Python []string `json:"python,omitempty"`
	NodeJS []string `json:"nodejs,omitempty"`
}

type WebhookScopeResponse struct {
	Route           string   `json:"route"`
	AuthStrategy    string   `json:"auth_strategy"`
	Header          string   `json:"header"`
	SecretRef       string   `json:"secret_ref"`
	SignaturePrefix string   `json:"signature_prefix,omitempty"`
	SourceIPs       []string `json:"source_ips,omitempty"`
}

type ScopesResponse struct {
	HTTPHosts    []string               `json:"http_hosts,omitempty"`
	StorageRoots []string               `json:"storage_roots,omitempty"`
	Webhooks     []WebhookScopeResponse `json:"webhooks,omitempty"`
}

type ScreenshotResponse struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUIResponse struct {
	Pages []ManagementUIPageResponse `json:"pages"`
}

type ManagementUIPageResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Entry string `json:"entry"`
}

type RenderTemplateResponse struct {
	Path string `json:"path"`
}

type DetailPluginResponse struct {
	ID                   string                   `json:"id"`
	Name                 string                   `json:"name"`
	Role                 string                   `json:"role"`
	Version              string                   `json:"version,omitempty"`
	Runtime              string                   `json:"runtime,omitempty"`
	Type                 string                   `json:"type,omitempty"`
	Entry                string                   `json:"entry,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Author               string                   `json:"author,omitempty"`
	License              string                   `json:"license,omitempty"`
	SDKMinVersion        string                   `json:"sdk_min_version,omitempty"`
	RuntimeVersion       string                   `json:"runtime_version,omitempty"`
	MinCoreVersion       string                   `json:"min_core_version,omitempty"`
	DataSchemaVersion    string                   `json:"data_schema_version,omitempty"`
	Concurrency          int                      `json:"concurrency,omitempty"`
	Platforms            []string                 `json:"platforms,omitempty"`
	DefaultConfig        map[string]any           `json:"default_config,omitempty"`
	DeclaredCapabilities []string                 `json:"declared_capabilities,omitempty"`
	Dependencies         *DependenciesResponse    `json:"dependencies,omitempty"`
	Scopes               *ScopesResponse          `json:"scopes,omitempty"`
	Icon                 string                   `json:"icon,omitempty"`
	Repo                 string                   `json:"repo,omitempty"`
	Homepage             string                   `json:"homepage,omitempty"`
	Keywords             []string                 `json:"keywords,omitempty"`
	Screenshots          []ScreenshotResponse     `json:"screenshots,omitempty"`
	ManagementUI         *ManagementUIResponse    `json:"management_ui,omitempty"`
	RenderTemplates      []RenderTemplateResponse `json:"render_templates,omitempty"`
	SystemDependencies   []string                 `json:"system_dependencies,omitempty"`
	RegistrationState    string                   `json:"registration_state"`
	DesiredState         string                   `json:"desired_state"`
	RuntimeState         string                   `json:"runtime_state"`
	DisplayState         string                   `json:"display_state"`
	Source               SourceResponse           `json:"source"`
	Trust                TrustResponse            `json:"trust"`
	Commands             []CommandResponse        `json:"commands"`
	Help                 HelpResponse             `json:"help"`
	CommandConflicts     []string                 `json:"command_conflicts"`
	DeadLetter           *DeadLetterResponse      `json:"dead_letter,omitempty"`
	Permissions          []PermissionResponse     `json:"permissions"`
}

type DetailResponse struct {
	Plugin DetailPluginResponse `json:"plugin"`
}
