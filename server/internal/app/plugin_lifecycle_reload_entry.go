package app

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) Reload(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if c.refreshManifest != nil {
		refreshed, err := c.refreshManifest(ctx, pluginID)
		if err != nil {
			return plugins.Snapshot{}, err
		}
		snapshot = refreshed
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			return plugins.Snapshot{}, plugins.ErrStateConflict
		}
	}

	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return plugins.Snapshot{}, err
	}

	if c.syncRenderTemplates != nil {
		if err := c.syncRenderTemplates(ctx); err != nil {
			return plugins.Snapshot{}, err
		}
	}

	updated, err := c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	if err != nil {
		updated = snapshot
	}

	taskID := c.createReloadTask(pluginID, snapshot)
	go c.reloadPluginAsync(pluginID, c.currentBotID(), taskID)
	c.reconcileRecoverySummaryBestEffort("plugin.reload")
	return updated, nil
}
