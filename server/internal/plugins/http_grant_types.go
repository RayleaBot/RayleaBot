package plugins

import "regexp"

type grantRequest struct {
	Capability string  `json:"capability"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

type grantResponse struct {
	PluginID   string  `json:"plugin_id"`
	Capability string  `json:"capability"`
	GrantedAt  *string `json:"granted_at"`
	Source     string  `json:"source"`
	ExpiresAt  *string `json:"expires_at"`
}

type grantsListResponse struct {
	Items []grantResponse `json:"items"`
}

// capabilityNamePattern matches the frozen multi-segment capability_name format from contracts/plugin-info.schema.json.
var capabilityNamePattern = regexp.MustCompile(`^[a-z]+(?:\.[a-z_]+)+$`)

type autoGrantCapabilitiesProvider func() []string
