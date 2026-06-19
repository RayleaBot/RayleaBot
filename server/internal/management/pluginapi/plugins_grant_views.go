package pluginapi

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi/view"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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

func buildPluginDetailResponse(ctx context.Context, catalog plugins.CatalogView, snapshot plugins.Snapshot, repo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) (view.DetailResponse, error) {
	persisted, err := loadPersistedGrants(ctx, repo, snapshot.PluginID)
	if err != nil {
		return view.DetailResponse{}, err
	}
	return view.BuildDetail(catalog, snapshot, persisted, providedAutoGrantCapabilities(autoGrantProvider)), nil
}
