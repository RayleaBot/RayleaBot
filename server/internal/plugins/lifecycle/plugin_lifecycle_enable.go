package lifecycle

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (c *Controller) Enable(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState == "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
		return plugins.Snapshot{}, err
	}

	updated, err := c.plugins.SetDesiredState(pluginID, "enabled")
	if err != nil {
		return plugins.Snapshot{}, err
	}

	if runtimeSnapshot, runtimeErr := c.plugins.SetRuntimeState(updated.PluginID, string(runtimemanager.StateStarting)); runtimeErr == nil {
		updated = runtimeSnapshot
	}
	go c.startPluginAsync(updated.PluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.enable")

	return updated, nil
}
