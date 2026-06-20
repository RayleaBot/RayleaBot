package lifecycle

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (c *Controller) RecoverFromDeadLetter(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}
	if manager.Snapshot().State != runtimemanager.StateDeadLetter {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}

	// Persist desired_state and update the catalog before mutating the
	// runtime manager. If persistence or catalog updates fail, the manager
	// must stay in dead_letter so a retry can pick the plugin up cleanly;
	// resetting the manager up front would leave the catalog reporting
	// dead_letter while the manager has already moved to stopped, which
	// would cause subsequent recovery attempts to fail with
	// plugin.not_in_dead_letter.
	updated := snapshot
	if snapshot.DesiredState != "enabled" {
		if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
			return plugins.Snapshot{}, err
		}
		if reEnabled, setErr := c.plugins.SetDesiredState(pluginID, "enabled"); setErr == nil {
			updated = reEnabled
		}
	}

	manager.ResetCrashCount()
	manager.SetStopped()

	if startingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStarting)); runtimeErr == nil {
		updated = startingSnapshot
	}

	go c.startPluginAsync(updated.PluginID, c.currentBotID())
	c.reconcileRecoverySummaryBestEffort("plugin.dead_letter_recover")
	return updated, nil
}
