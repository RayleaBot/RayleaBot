package lifecycle

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
)

func (c *Controller) HandleAdapterReady(ctx context.Context) {
	c.reconcileRuntime(ctx)
	c.SyncBotIdentities(ctx)
}

// SyncBotIdentities serializes snapshot capture and admission so concurrent
// adapter callbacks cannot publish an older identity list after a newer one.
func (c *Controller) SyncBotIdentities(ctx context.Context) {
	if c.dispatcher == nil {
		return
	}
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	bots := c.botIdentities()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string][]chatevent.BotIdentity)
	}
	for _, pluginID := range c.dispatcher.PluginIDs() {
		previous, sent := c.identityByPlugin[pluginID]
		if sent && slices.Equal(previous, bots) {
			continue
		}
		now := time.Now()
		event := chatevent.Event{
			EventID:        fmt.Sprintf("bot-identities-%d", now.UnixNano()),
			SourceProtocol: "platform", SourceAdapter: "adapters.internal",
			EventType: "bot.identities.changed", Timestamp: now.Unix(),
			PayloadFields: map[string]any{"bots": append([]chatevent.BotIdentity{}, bots...)},
		}
		result := c.dispatcher.DispatchToPlugin(ctx, pluginID, event)
		if result.Outcome == dispatch.OutcomeDelivered {
			c.identityByPlugin[pluginID] = append([]chatevent.BotIdentity{}, bots...)
		}
	}
}

func (c *Controller) clearBotIdentity(pluginID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	delete(c.identityByPlugin, pluginID)
}

func (c *Controller) botIdentities() []chatevent.BotIdentity {
	if c.identities == nil {
		return []chatevent.BotIdentity{}
	}
	return c.identities.BotIdentities()
}
