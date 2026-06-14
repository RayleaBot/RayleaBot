package managementhttp

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func parseGrantRequestExpiry(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	raw := strings.TrimSpace(*value)
	if raw == "" || !strings.HasSuffix(raw, "Z") {
		return nil, errors.New("expires_at must be a UTC RFC3339 timestamp")
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	if !parsed.After(time.Now().UTC()) {
		return nil, errors.New("expires_at must be in the future")
	}
	return &parsed, nil
}

// isCapabilityDeclared checks whether a capability is declared in the plugin's
// manifest via capabilities, permissions.required, or permissions.optional.
func isCapabilityDeclared(snapshot Snapshot, capability string) bool {
	for _, c := range snapshot.DeclaredCapabilities {
		if c == capability {
			return true
		}
	}
	for _, c := range snapshot.RequiredPermissions {
		if c == capability {
			return true
		}
	}
	for _, c := range snapshot.OptionalPermissions {
		if c == capability {
			return true
		}
	}
	return false
}

// BuildScopeJSON constructs a JSON string from the plugin manifest's scope
// boundaries for persistence alongside the grant.
func BuildScopeJSON(snapshot Snapshot) string {
	if len(snapshot.ScopeHTTPHosts) == 0 && len(snapshot.ScopeStorageRoots) == 0 && len(snapshot.ScopeWebhooks) == 0 {
		return ""
	}
	scope := map[string]any{}
	if len(snapshot.ScopeHTTPHosts) > 0 {
		scope["http_hosts"] = snapshot.ScopeHTTPHosts
	}
	if len(snapshot.ScopeStorageRoots) > 0 {
		scope["storage_roots"] = snapshot.ScopeStorageRoots
	}
	if len(snapshot.ScopeWebhooks) > 0 {
		scope["webhooks"] = snapshot.ScopeWebhooks
	}
	data, err := json.Marshal(scope)
	if err != nil {
		return ""
	}
	return string(data)
}
