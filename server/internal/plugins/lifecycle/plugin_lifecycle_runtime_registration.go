package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (c *Controller) afterRuntimeRegistered(ctx context.Context, pluginID string, initBotID string) {
	c.dispatchPluginStarted(ctx, pluginID)

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

func (c *Controller) registerRuntimeIfNeeded(pluginID string, manager *runtimemanager.Manager) {
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

func (c *Controller) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *runtimemanager.Manager) {
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

func (c *Controller) dispatchPluginStarted(ctx context.Context, pluginID string) {
	if c == nil || c.dispatcher == nil {
		return
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return
	}

	now := time.Now()
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtimeprotocol.Event{
		EventID:        fmt.Sprintf("plugin-started-%s-%d", pluginID, now.UnixNano()),
		SourceProtocol: "platform",
		SourceAdapter:  "plugin.lifecycle",
		EventType:      "plugin.started",
		Timestamp:      now.Unix(),
	})
	if result.Outcome == dispatch.OutcomeDelivered || c.logger == nil {
		return
	}
	pluginLabel := pluginID
	pluginName := ""
	if c.plugins != nil {
		if snapshot, ok := c.plugins.Get(pluginID); ok {
			pluginLabel = plugins.DisplayLabel(snapshot)
			pluginName = snapshot.Name
		}
	}
	c.logger.Warn(
		"插件"+pluginLabel+"启动事件投递失败",
		"component", "app",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}
