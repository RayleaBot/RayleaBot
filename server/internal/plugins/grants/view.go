package grants

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

type View struct {
	plugins               *plugincatalog.Catalog
	grantRepository       plugins.GrantRepository
	autoGrantCapabilities func() []string
}

type ViewDeps struct {
	Plugins               *plugincatalog.Catalog
	GrantRepository       plugins.GrantRepository
	AutoGrantCapabilities func() []string
}

func NewView(deps ViewDeps) *View {
	return &View{
		plugins:               deps.Plugins,
		grantRepository:       deps.GrantRepository,
		autoGrantCapabilities: deps.AutoGrantCapabilities,
	}
}

func (v *View) GrantedCapabilities(ctx context.Context, pluginID string) []string {
	effective := v.effectiveGrants(ctx, pluginID)
	items := make([]string, 0, len(effective))
	for _, grant := range effective {
		items = append(items, grant.Capability)
	}
	return items
}

func (v *View) CapabilityGranted(ctx context.Context, pluginID, capability string) bool {
	for _, granted := range v.GrantedCapabilities(ctx, pluginID) {
		if strings.TrimSpace(granted) == capability {
			return true
		}
	}
	return false
}

func (v *View) grantedScope(ctx context.Context, pluginID, capability string) grantedScope {
	for _, grant := range v.effectiveGrants(ctx, pluginID) {
		if strings.TrimSpace(grant.Capability) != capability {
			continue
		}
		scope := parseGrantedScope(grant.ScopeJSON)
		if len(scope.HTTPHosts) > 0 || len(scope.StorageRoots) > 0 || len(scope.Webhooks) > 0 {
			return scope
		}
	}

	return grantedScope{}
}

func (v *View) effectiveGrants(ctx context.Context, pluginID string) []plugins.EffectiveGrant {
	if v == nil {
		return nil
	}

	snapshot := plugins.Snapshot{PluginID: pluginID}
	if v.plugins != nil {
		if current, ok := v.plugins.Get(pluginID); ok {
			snapshot = current
		}
	}

	var persisted []plugins.PluginGrant
	if v.grantRepository != nil {
		grants, err := v.grantRepository.LoadGrants(ctx, pluginID)
		if err == nil {
			persisted = grants
		}
	}

	return plugins.ComputeEffectiveGrants(snapshot, currentAutoGrantCapabilities(v), persisted)
}

func currentAutoGrantCapabilities(v *View) []string {
	if v == nil || v.autoGrantCapabilities == nil {
		return nil
	}
	return append([]string(nil), v.autoGrantCapabilities()...)
}

func (v *View) StorageRootGranted(ctx context.Context, pluginID, root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	for _, grantedRoot := range v.grantedScope(ctx, pluginID, "storage.file").StorageRoots {
		if strings.TrimSpace(grantedRoot) == root {
			return true
		}
	}
	return false
}

func (v *View) GrantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	scope := v.grantedScope(ctx, pluginID, "event.expose_webhook")
	route = strings.TrimSpace(route)
	for _, item := range scope.Webhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return plugins.WebhookScope{}, false
}

func (v *View) GrantedHTTPHosts(ctx context.Context, pluginID string) []string {
	return append([]string(nil), v.grantedScope(ctx, pluginID, "http.request").HTTPHosts...)
}

func (v *View) ListPluginSnapshots() []plugins.Snapshot {
	if v == nil || v.plugins == nil {
		return nil
	}
	return v.plugins.List()
}

func (v *View) ScopeChangedSinceGrant(ctx context.Context, snapshot plugins.Snapshot) bool {
	if v == nil || v.grantRepository == nil {
		return false
	}
	return ScopeChangedSinceGrant(ctx, v.grantRepository, snapshot)
}

func ScopeChangedSinceGrant(ctx context.Context, repo plugins.GrantRepository, snapshot plugins.Snapshot) bool {
	grants, err := repo.LoadGrants(ctx, snapshot.PluginID)
	if err != nil || len(grants) == 0 {
		return false
	}
	currentScope := plugins.BuildScopeJSON(snapshot)
	for _, grant := range grants {
		if grant.ScopeJSON != currentScope {
			return true
		}
	}
	return false
}

type grantedScope struct {
	HTTPHosts    []string               `json:"http_hosts"`
	StorageRoots []string               `json:"storage_roots"`
	Webhooks     []plugins.WebhookScope `json:"webhooks"`
}

func parseGrantedScope(raw string) grantedScope {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return grantedScope{}
	}
	var scope grantedScope
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return grantedScope{}
	}
	return scope
}
