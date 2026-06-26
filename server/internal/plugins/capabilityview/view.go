package capabilityview

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type View struct {
	plugins plugins.CatalogView
}

type Deps struct {
	Plugins plugins.CatalogView
}

func New(deps Deps) *View {
	return &View{plugins: deps.Plugins}
}

func (v *View) DeclaredCapabilities(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return plugins.DedupeCapabilities(snapshot.DeclaredCapabilities)
}

func (v *View) CapabilityDeclared(ctx context.Context, pluginID, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	for _, declared := range v.DeclaredCapabilities(ctx, pluginID) {
		if declared == capability {
			return true
		}
	}
	return false
}

func (v *View) HTTPHosts(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return append([]string(nil), snapshot.ScopeHTTPHosts...)
}

func (v *View) ThirdPartyAccountPlatforms(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return append([]string(nil), snapshot.ScopeThirdPartyAccounts...)
}

func (v *View) StorageRootAllowed(ctx context.Context, pluginID, root string) bool {
	_ = ctx
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return false
	}
	for _, declared := range snapshot.ScopeStorageRoots {
		if strings.TrimSpace(declared) == root {
			return true
		}
	}
	return false
}

func (v *View) WebhookParameters(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	_ = ctx
	route = strings.TrimSpace(route)
	if route == "" {
		return plugins.WebhookScope{}, false
	}
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return plugins.WebhookScope{}, false
	}
	for _, item := range snapshot.ScopeWebhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return plugins.WebhookScope{}, false
}

func (v *View) ListPluginSnapshots() []plugins.Snapshot {
	if v == nil || v.plugins == nil {
		return nil
	}
	return v.plugins.List()
}

func (v *View) snapshot(pluginID string) (plugins.Snapshot, bool) {
	if v == nil || v.plugins == nil {
		return plugins.Snapshot{}, false
	}
	return v.plugins.Get(pluginID)
}
