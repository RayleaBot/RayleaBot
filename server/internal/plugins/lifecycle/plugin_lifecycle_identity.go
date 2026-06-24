package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (c *Controller) HandleAdapterReady(ctx context.Context) {
	if c == nil {
		return
	}
	botID := c.currentBotID()
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *Controller) HandleAdapterBotID(ctx context.Context, botID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *Controller) broadcastBotIdentityChanged(ctx context.Context, botID string) {
	if c == nil || c.dispatcher == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return
	}
	for _, pluginID := range c.dispatcher.PluginIDs() {
		c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, botID)
	}
}

func (c *Controller) dispatchBotIdentityChangedToPlugin(ctx context.Context, pluginID string, botID string) {
	if c == nil || c.dispatcher == nil {
		return
	}
	pluginID = strings.TrimSpace(pluginID)
	botID = strings.TrimSpace(botID)
	if pluginID == "" || botID == "" {
		return
	}
	if c.botIdentityAlreadySent(pluginID, botID) {
		return
	}

	now := time.Now()
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtimeprotocol.Event{
		EventID:        fmt.Sprintf("onebot11-bot-identity-%d-%s", now.UnixNano(), botID),
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "bot.identity.changed",
		Timestamp:      now.Unix(),
		Target: &runtimeprotocol.EventTarget{
			Type: "bot",
			ID:   botID,
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"self_id": botID,
				"time":    now.Unix(),
			},
		},
	})
	if result.Outcome == dispatch.OutcomeDelivered {
		c.markBotIdentitySent(pluginID, botID)
	}
}

func (c *Controller) botIdentityAlreadySent(pluginID string, botID string) bool {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	return c.identityByPlugin != nil && c.identityByPlugin[pluginID] == botID
}

func (c *Controller) markBotIdentitySent(pluginID string, botID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string]string)
	}
	c.identityByPlugin[pluginID] = botID
}

func (c *Controller) clearBotIdentity(pluginID string) {
	if c == nil {
		return
	}
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin != nil {
		delete(c.identityByPlugin, pluginID)
	}
}

func (c *Controller) currentBotID() string {
	if c == nil || c.adapter == nil {
		return ""
	}
	return strings.TrimSpace(c.adapter.CurrentBotID())
}

func (c *Controller) CurrentBotID() string {
	return c.currentBotID()
}
