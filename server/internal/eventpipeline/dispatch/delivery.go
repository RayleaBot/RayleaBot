package dispatch

import (
	"context"

	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

// Dispatch fans out an event to all matching registered plugins.
// If commandName is non-empty, plugins declaring that command are
// preferred (directed delivery). Otherwise all message-subscribed
// plugins receive the event.
func (d *Dispatcher) Dispatch(ctx context.Context, event runtimeprotocol.Event, commandName string) []DeliveryResult {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	targets := d.selectTargets(event, commandName)
	d.mu.RUnlock()

	if len(targets) == 0 {
		d.recordOutcome(OutcomeIgnored, "", "")
		return nil
	}

	return d.enqueueTargets(ctx, event, targets)
}

// DispatchToPlugin delivers an event to one specific registered plugin.
func (d *Dispatcher) DispatchToPlugin(ctx context.Context, pluginID string, event runtimeprotocol.Event) DeliveryResult {
	if d == nil {
		return DeliveryResult{
			PluginID:  pluginID,
			Outcome:   OutcomeError,
			ErrorCode: "platform.invalid_request",
		}
	}

	results := d.enqueueTargets(ctx, event, []string{pluginID})
	if len(results) == 0 {
		return DeliveryResult{
			PluginID:  pluginID,
			Outcome:   OutcomeError,
			ErrorCode: "platform.invalid_request",
		}
	}
	return results[0]
}
func (d *Dispatcher) enqueueTargets(ctx context.Context, event runtimeprotocol.Event, targets []string) []DeliveryResult {
	results := make([]DeliveryResult, 0, len(targets))
	for _, pluginID := range targets {
		d.mu.RLock()
		slot, ok := d.slots[pluginID]
		deliverable := ok && slotIsDeliverable(slot)
		d.mu.RUnlock()
		if !ok || !deliverable {
			results = append(results, DeliveryResult{
				PluginID:  pluginID,
				Outcome:   OutcomeError,
				ErrorCode: "platform.invalid_request",
			})
			d.recordOutcome(OutcomeDropped, pluginID, "plugin_not_running")
			continue
		}

		item := dispatchItem{ctx: ctx, event: event}
		select {
		case slot.queue <- item:
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDelivered})
			d.recordOutcome(OutcomeDelivered, pluginID, "")
		default:
			d.logger.Warn("插件 "+pluginID+" 的事件队列已满，已丢弃事件："+event.EventID,
				"component", "dispatch",
				"plugin_id", pluginID,
				"event_id", event.EventID,
			)
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDropped})
			d.recordOutcome(OutcomeDropped, pluginID, "queue_full")
		}
	}
	return results
}

// selectTargets picks which plugins should receive the event.
// Must be called with d.mu held for reading.
func (d *Dispatcher) selectTargets(event runtimeprotocol.Event, commandName string) []string {
	// If there's a command, try directed delivery first.
	if commandName != "" {
		var directed []string
		for id, slot := range d.slots {
			if !slotIsDeliverable(slot) {
				continue
			}
			if slotDeclaresCommand(slot, commandName) {
				directed = append(directed, id)
			}
		}
		if len(directed) > 0 {
			return directed
		}
	}

	// Fan-out to all plugins with matching subscriptions.
	var targets []string
	for id, slot := range d.slots {
		if !slotIsDeliverable(slot) {
			continue
		}
		if slotAcceptsEvent(slot, event.EventType) {
			targets = append(targets, id)
		}
	}
	return targets
}
func slotDeclaresCommand(slot *pluginSlot, commandName string) bool {
	for _, cmd := range slot.commands {
		if cmd.Name == commandName {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == commandName {
				return true
			}
		}
	}
	return false
}
func slotIsDeliverable(slot *pluginSlot) bool {
	if slot == nil || slot.runtime == nil {
		return false
	}
	return slot.runtime.Snapshot().State == runtimemanager.StateRunning
}
func slotAcceptsEvent(slot *pluginSlot, eventType string) bool {
	// No subscriptions means accept all events.
	if len(slot.subscriptions) == 0 {
		return true
	}
	for _, sub := range slot.subscriptions {
		if sub == eventType || sub == "*" {
			return true
		}
	}
	return false
}
