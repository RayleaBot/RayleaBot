package actions_test

import (
	"context"
	"encoding/json"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubPermission struct {
	PluginID   string
	Permission string
	ScopeJSON  string
}

type scopedPermissionView struct {
	permissions map[string][]stubPermission
}

func scopedPermissionViewFor(pluginID string, permissions ...string) *scopedPermissionView {
	view := &scopedPermissionView{permissions: map[string][]stubPermission{}}
	for _, permission := range permissions {
		view.permissions[pluginID] = append(view.permissions[pluginID], stubPermission{
			PluginID:   pluginID,
			Permission: permission,
		})
	}
	return view
}

func (v *scopedPermissionView) PermissionDeclared(_ context.Context, pluginID string, permission string) bool {
	if v == nil {
		return false
	}
	for _, item := range v.permissions[pluginID] {
		if item.Permission == permission {
			return true
		}
	}
	return false
}

func (v *scopedPermissionView) PermissionPlatforms(_ context.Context, pluginID, permission string) []string {
	for _, item := range v.permissions[pluginID] {
		if item.Permission == permission {
			return parseStubScopeList(item.ScopeJSON, "third_party_account_platforms")
		}
	}
	return nil
}

func (v *scopedPermissionView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}

func parseStubScopeList(scopeJSON string, key string) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(scopeJSON), &payload); err != nil {
		return nil
	}
	raw, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && value != "" {
			values = append(values, value)
		}
	}
	return values
}

func defaultAdapterTestConfig() config.AdapterConfig {
	return config.AdapterConfig{
		ConnectTimeoutSeconds:   15,
		ReconnectInitialSeconds: 2,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     120,
		ReconnectJitterRatio:    0.2,
	}
}
