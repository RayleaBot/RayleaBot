package pluginapi

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

type stubGrantRepository struct {
	grants map[string][]plugins.PluginGrant
}

func (r *stubGrantRepository) LoadGrants(_ context.Context, pluginID string) ([]plugins.PluginGrant, error) {
	now := time.Now().UTC()
	var active []plugins.PluginGrant
	for _, grant := range r.grants[pluginID] {
		if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
			continue
		}
		active = append(active, grant)
	}
	return active, nil
}

func (r *stubGrantRepository) LoadAllGrants(_ context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	for pid := range r.grants {
		gs, _ := r.LoadGrants(context.Background(), pid)
		for _, g := range gs {
			result[pid] = append(result[pid], g.Capability)
		}
	}
	return result, nil
}

func (r *stubGrantRepository) SaveGrant(_ context.Context, grant plugins.PluginGrant) error {
	if r.grants == nil {
		r.grants = make(map[string][]plugins.PluginGrant)
	}
	items := r.grants[grant.PluginID]
	for i, existing := range items {
		if existing.Capability == grant.Capability {
			items[i] = grant
			r.grants[grant.PluginID] = items
			return nil
		}
	}
	r.grants[grant.PluginID] = append(items, grant)
	return nil
}

func (r *stubGrantRepository) DeleteGrant(_ context.Context, pluginID, capability string) error {
	gs := r.grants[pluginID]
	for i, g := range gs {
		if g.Capability == capability {
			r.grants[pluginID] = append(gs[:i], gs[i+1:]...)
			break
		}
	}
	return nil
}

func (r *stubGrantRepository) DeleteAllGrants(_ context.Context, pluginID string) error {
	delete(r.grants, pluginID)
	return nil
}

func grantsRouter(entries []plugins.Snapshot, grantRepo plugins.GrantRepository) chi.Router {
	return grantsRouterWithAutoGrants(entries, grantRepo, nil)
}

func grantsRouterWithAutoGrants(entries []plugins.Snapshot, grantRepo plugins.GrantRepository, autoGrants []string) chi.Router {
	catalog := newTestCatalog(entries)
	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, nil, nil, grantRepo, func() []string {
		return append([]string(nil), autoGrants...)
	})
	return router
}
