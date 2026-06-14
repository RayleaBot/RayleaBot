package pluginapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"time"
)

func providedAutoGrantCapabilities(provider autoGrantCapabilitiesProvider) []string {
	if provider == nil {
		return nil
	}
	return plugins.DedupeCapabilities(provider())
}

func loadPersistedGrants(ctx context.Context, repo plugins.GrantRepository, pluginID string) ([]plugins.PluginGrant, error) {
	if repo == nil {
		return nil, nil
	}
	return repo.LoadGrants(ctx, pluginID)
}

func buildGrantResponses(grants []plugins.EffectiveGrant) []grantResponse {
	if len(grants) == 0 {
		return []grantResponse{}
	}

	items := make([]grantResponse, 0, len(grants))
	for _, grant := range grants {
		response := grantResponse{
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

func buildPermissionResponses(summaries []plugins.PermissionSummary) []pluginPermissionResponse {
	if len(summaries) == 0 {
		return []pluginPermissionResponse{}
	}

	items := make([]pluginPermissionResponse, 0, len(summaries))
	for _, summary := range summaries {
		item := pluginPermissionResponse{
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
