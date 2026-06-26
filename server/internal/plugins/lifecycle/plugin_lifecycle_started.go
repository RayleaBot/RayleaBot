package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

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
	c.logger.Warn(
		"plugin started event delivery failed",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}
