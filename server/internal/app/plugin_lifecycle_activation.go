package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (c *pluginLifecycleController) validateActivation(ctx context.Context, snapshot plugins.Snapshot) ([]string, error) {
	granted := c.grants.grantedCapabilities(ctx, snapshot.PluginID)
	if missing := missingCapabilities(snapshot.RequiredPermissions, granted); len(missing) > 0 {
		return granted, &plugins.PermissionPendingError{
			PluginID:            snapshot.PluginID,
			MissingCapabilities: missing,
		}
	}

	if c.grants != nil && c.grants.grantRepository != nil {
		if changed := scopeChangedSinceGrant(ctx, c.grants.grantRepository, snapshot); changed {
			return granted, &plugins.PermissionPendingError{
				PluginID:     snapshot.PluginID,
				ScopeChanged: true,
			}
		}
	}

	return granted, nil
}
