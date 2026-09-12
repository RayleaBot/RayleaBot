package actions_test

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubPermissionView struct {
	permissions map[string]map[string]bool
}

func permissionViewFor(pluginID string, permissions ...string) *stubPermissionView {
	view := &stubPermissionView{permissions: map[string]map[string]bool{pluginID: {}}}
	for _, permission := range permissions {
		view.permissions[pluginID][permission] = true
	}
	return view
}

func (v *stubPermissionView) PermissionDeclared(_ context.Context, pluginID string, permission string) bool {
	return v != nil && v.permissions[pluginID][permission]
}

func (v *stubPermissionView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
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
