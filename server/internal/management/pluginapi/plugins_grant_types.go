package pluginapi

import "regexp"

type grantRequest struct {
	Capability string  `json:"capability"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

// capabilityNamePattern matches the frozen multi-segment capability_name format from contracts/plugin-info.schema.json.
var capabilityNamePattern = regexp.MustCompile(`^[a-z]+(?:\.[a-z_]+)+$`)

type autoGrantCapabilitiesProvider func() []string
