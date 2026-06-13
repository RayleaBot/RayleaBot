package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) HandleAdapterReady(ctx context.Context) {
	if c == nil {
		return
	}
	botID := c.currentBotID()
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *pluginLifecycleController) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if c == nil {
		return
	}
	botID := strings.TrimSpace(event.BotID)
	c.reconcileRuntime(ctx, botID)
	c.broadcastBotIdentityChanged(ctx, botID)
}

func (c *pluginLifecycleController) broadcastBotIdentityChanged(ctx context.Context, botID string) {
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

func (c *pluginLifecycleController) dispatchBotIdentityChangedToPlugin(ctx context.Context, pluginID string, botID string) {
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
	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("onebot11-bot-identity-%d-%s", now.UnixNano(), botID),
		SourceProtocol: "onebot11",
		SourceAdapter:  "adapter.onebot11",
		EventType:      "bot.identity.changed",
		Timestamp:      now.Unix(),
		Target: &runtime.EventTarget{
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

func (c *pluginLifecycleController) botIdentityAlreadySent(pluginID string, botID string) bool {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	return c.identityByPlugin != nil && c.identityByPlugin[pluginID] == botID
}

func (c *pluginLifecycleController) markBotIdentitySent(pluginID string, botID string) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin == nil {
		c.identityByPlugin = make(map[string]string)
	}
	c.identityByPlugin[pluginID] = botID
}

func (c *pluginLifecycleController) clearBotIdentity(pluginID string) {
	if c == nil {
		return
	}
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.identityByPlugin != nil {
		delete(c.identityByPlugin, pluginID)
	}
}

func (c *pluginLifecycleController) currentBotID() string {
	if c == nil || c.adapter == nil {
		return ""
	}
	snapshot := c.adapter.Snapshot()
	if snapshot.State != adapter.StateConnected {
		return ""
	}
	return strings.TrimSpace(snapshot.BotID)
}

func (c *pluginLifecycleController) CurrentBotID() string {
	return c.currentBotID()
}
