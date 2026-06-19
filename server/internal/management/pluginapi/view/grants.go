package view

import (
	"encoding/json"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type GrantResponse struct {
	PluginID   string  `json:"plugin_id"`
	Capability string  `json:"capability"`
	GrantedAt  *string `json:"granted_at"`
	Source     string  `json:"source"`
	ExpiresAt  *string `json:"expires_at"`
}

type GrantsListResponse struct {
	Items []GrantResponse `json:"items"`
}

func BuildGrantResponses(grants []plugins.EffectiveGrant) []GrantResponse {
	if len(grants) == 0 {
		return []GrantResponse{}
	}

	items := make([]GrantResponse, 0, len(grants))
	for _, grant := range grants {
		response := GrantResponse{
			PluginID:   grant.PluginID,
			Capability: grant.Capability,
			Source:     string(grant.Source),
		}
		if grant.GrantedAt != nil {
			value := grant.GrantedAt.UTC().Format(time.RFC3339)
			response.GrantedAt = &value
		}
		if grant.ExpiresAt != nil {
			value := grant.ExpiresAt.UTC().Format(time.RFC3339)
			response.ExpiresAt = &value
		}
		items = append(items, response)
	}
	return items
}

func BuildPermissionResponses(summaries []plugins.PermissionSummary) []PermissionResponse {
	if len(summaries) == 0 {
		return []PermissionResponse{}
	}

	items := make([]PermissionResponse, 0, len(summaries))
	for _, summary := range summaries {
		item := PermissionResponse{
			Capability:  summary.Capability,
			Requirement: string(summary.Requirement),
			Status:      string(summary.Status),
			Source:      string(summary.Source),
		}
		if summary.ExpiresAt != nil {
			value := summary.ExpiresAt.UTC().Format(time.RFC3339)
			item.ExpiresAt = &value
		}
		items = append(items, item)
	}
	return items
}

func IsCapabilityDeclared(snapshot plugins.Snapshot, capability string) bool {
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

func BuildScopeJSON(snapshot plugins.Snapshot) string {
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
