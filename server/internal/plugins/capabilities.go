package plugins

import (
	"context"
	"strings"
)

type CapabilityView struct {
	plugins CatalogView
}

type CapabilityViewDeps struct {
	Plugins CatalogView
}

func NewCapabilityView(deps CapabilityViewDeps) *CapabilityView {
	return &CapabilityView{plugins: deps.Plugins}
}

func (v *CapabilityView) DeclaredCapabilities(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return DedupeCapabilities(snapshot.DeclaredCapabilities)
}

func (v *CapabilityView) CapabilityDeclared(ctx context.Context, pluginID, capability string) bool {
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

func (v *CapabilityView) HTTPHosts(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return append([]string(nil), snapshot.ScopeHTTPHosts...)
}

func (v *CapabilityView) ThirdPartyAccountPlatforms(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	return append([]string(nil), snapshot.ScopeThirdPartyAccounts...)
}

func (v *CapabilityView) StorageRootAllowed(ctx context.Context, pluginID, root string) bool {
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

func (v *CapabilityView) WebhookParameters(ctx context.Context, pluginID, route string) (WebhookScope, bool) {
	_ = ctx
	route = strings.TrimSpace(route)
	if route == "" {
		return WebhookScope{}, false
	}
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return WebhookScope{}, false
	}
	for _, item := range snapshot.ScopeWebhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return WebhookScope{}, false
}

func (v *CapabilityView) ListPluginSnapshots() []Snapshot {
	if v.plugins == nil {
		return nil
	}
	return v.plugins.List()
}

func (v *CapabilityView) snapshot(pluginID string) (Snapshot, bool) {
	if v.plugins == nil {
		return Snapshot{}, false
	}
	return v.plugins.Get(pluginID)
}

func dedupeCapabilities(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func DedupeCapabilities(values []string) []string {
	return dedupeCapabilities(values)
}

func CapabilityDeclared(snapshot Snapshot, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	for _, declared := range snapshot.DeclaredCapabilities {
		if strings.TrimSpace(declared) == capability {
			return true
		}
	}
	return false
}
