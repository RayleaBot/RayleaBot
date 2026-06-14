package service

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *Controller) afterRuntimeRegistered(ctx context.Context, pluginID string, initBotID string) {
	initBotID = strings.TrimSpace(initBotID)
	currentBotID := c.currentBotID()
	if initBotID != "" {
		c.markBotIdentitySent(pluginID, initBotID)
		if currentBotID != "" && currentBotID != initBotID {
			c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
		}
		return
	}
	c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
}

func (c *Controller) registerRuntimeIfNeeded(pluginID string, manager *runtime.Manager) {
	if c == nil || c.dispatcher == nil || manager == nil {
		return
	}
	if c.dispatcher.HasDeliverablePlugin(pluginID) {
		return
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return
	}
	c.registerRuntime(pluginID, snapshot, manager)
}

func (c *Controller) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *runtime.Manager) {
	if c == nil || c.dispatcher == nil || manager == nil {
		return
	}
	runtimeSnapshot := manager.Snapshot()
	concurrency := snapshot.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if max := c.config().Runtime.MaxConcurrentTasksPerPlugin; max > 0 && concurrency > max {
		concurrency = max
	}
	c.dispatcher.Register(pluginID, manager, runtimeSnapshot.Subscriptions, dispatchCommands(snapshot.Commands), concurrency)
}
