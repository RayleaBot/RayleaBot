package plugins

import (
	"context"
	"sort"
	"strings"
)

type PermissionView struct {
	plugins CatalogView
}

type PermissionViewDeps struct {
	Plugins CatalogView
}

func NewPermissionView(deps PermissionViewDeps) *PermissionView {
	return &PermissionView{plugins: deps.Plugins}
}

func (v *PermissionView) DeclaredPermissions(ctx context.Context, pluginID string) []string {
	_ = ctx
	snapshot, ok := v.snapshot(pluginID)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(snapshot.Permissions))
	for name := range snapshot.Permissions {
		items = append(items, name)
	}
	sort.Strings(items)
	return items
}

func (v *PermissionView) PermissionDeclared(ctx context.Context, pluginID, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, declared := range v.DeclaredPermissions(ctx, pluginID) {
		if declared == permission {
			return true
		}
	}
	return false
}

func (v *PermissionView) ListPluginSnapshots() []Snapshot {
	if v.plugins == nil {
		return nil
	}
	return v.plugins.List()
}

func (v *PermissionView) snapshot(pluginID string) (Snapshot, bool) {
	if v.plugins == nil {
		return Snapshot{}, false
	}
	return v.plugins.Get(pluginID)
}
