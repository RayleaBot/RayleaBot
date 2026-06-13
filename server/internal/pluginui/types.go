package pluginui

type pluginSettingsRequest struct {
	Values map[string]any `json:"values"`
}

type pluginSecretsRequest struct {
	Values      map[string]string `json:"values"`
	DeletedKeys []string          `json:"deleted_keys,omitempty"`
}

type PluginSettingsResponse struct {
	PluginID string         `json:"plugin_id"`
	Values   map[string]any `json:"values"`
}

type PluginSettingsUpdateResponse struct {
	PluginID    string         `json:"plugin_id"`
	ChangedKeys []string       `json:"changed_keys"`
	Values      map[string]any `json:"values"`
}

type PluginSecretsResponse struct {
	PluginID string            `json:"plugin_id"`
	Values   map[string]string `json:"values"`
}

type PluginSecretsUpdateResponse struct {
	PluginID    string            `json:"plugin_id"`
	ChangedKeys []string          `json:"changed_keys"`
	Values      map[string]string `json:"values"`
}
