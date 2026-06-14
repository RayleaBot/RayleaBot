package service

import (
	"context"
	"encoding/json"
	"strings"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type GrantView struct {
	plugins               *plugincatalog.Catalog
	grantRepository       plugins.GrantRepository
	autoGrantCapabilities func() []string
}

type GrantViewDeps struct {
	Plugins               *plugincatalog.Catalog
	GrantRepository       plugins.GrantRepository
	AutoGrantCapabilities func() []string
}

func NewGrantView(deps GrantViewDeps) *GrantView {
	return &GrantView{
		plugins:               deps.Plugins,
		grantRepository:       deps.GrantRepository,
		autoGrantCapabilities: deps.AutoGrantCapabilities,
	}
}

func (v *GrantView) grantedCapabilities(ctx context.Context, pluginID string) []string {
	effective := v.effectiveGrants(ctx, pluginID)
	items := make([]string, 0, len(effective))
	for _, grant := range effective {
		items = append(items, grant.Capability)
	}
	return items
}

func (v *GrantView) capabilityGranted(ctx context.Context, pluginID, capability string) bool {
	for _, granted := range v.grantedCapabilities(ctx, pluginID) {
		if strings.TrimSpace(granted) == capability {
			return true
		}
	}
	return false
}

func (v *GrantView) CapabilityGranted(ctx context.Context, pluginID, capability string) bool {
	return v.capabilityGranted(ctx, pluginID, capability)
}

func (v *GrantView) grantedScope(ctx context.Context, pluginID, capability string) grantedScope {
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

func (v *GrantView) effectiveGrants(ctx context.Context, pluginID string) []plugins.EffectiveGrant {
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

func currentAutoGrantCapabilities(v *GrantView) []string {
	if v == nil || v.autoGrantCapabilities == nil {
		return nil
	}
	return append([]string(nil), v.autoGrantCapabilities()...)
}

func (v *GrantView) storageRootGranted(ctx context.Context, pluginID, root string) bool {
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

func (v *GrantView) StorageRootGranted(ctx context.Context, pluginID, root string) bool {
	return v.storageRootGranted(ctx, pluginID, root)
}

func (v *GrantView) grantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	scope := v.grantedScope(ctx, pluginID, "event.expose_webhook")
	route = strings.TrimSpace(route)
	for _, item := range scope.Webhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return plugins.WebhookScope{}, false
}

func (v *GrantView) GrantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	return v.grantedWebhookScope(ctx, pluginID, route)
}

func (v *GrantView) GrantedHTTPHosts(ctx context.Context, pluginID string) []string {
	return append([]string(nil), v.grantedScope(ctx, pluginID, "http.request").HTTPHosts...)
}

func (v *GrantView) ListPluginSnapshots() []plugins.Snapshot {
	if v == nil || v.plugins == nil {
		return nil
	}
	return v.plugins.List()
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
