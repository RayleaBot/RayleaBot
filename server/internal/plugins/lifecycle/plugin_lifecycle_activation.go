package lifecycle

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (c *Controller) validateActivation(ctx context.Context, snapshot plugins.Snapshot) ([]string, error) {
	granted := c.grants.GrantedCapabilities(ctx, snapshot.PluginID)
	if missing := missingCapabilities(snapshot.RequiredPermissions, granted); len(missing) > 0 {
		return granted, &plugins.PermissionPendingError{
			PluginID:            snapshot.PluginID,
			MissingCapabilities: missing,
		}
	}

	if c.grants != nil && c.grants.ScopeChangedSinceGrant(ctx, snapshot) {
		return granted, &plugins.PermissionPendingError{
			PluginID:     snapshot.PluginID,
			ScopeChanged: true,
		}
	}

	return granted, nil
}
