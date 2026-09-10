package dispatch

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

// Dispatch fans out an event to all matching registered plugins.
// If commandName is non-empty, plugins declaring that command are
// preferred (directed delivery). Otherwise all message-subscribed
// plugins receive the event.
func (d *Dispatcher) Dispatch(ctx context.Context, event chatevent.Event, commandName string) []DeliveryResult {

	d.mu.RLock()
	targets := d.selectTargets(event, commandName)
	d.mu.RUnlock()

	if len(targets) == 0 {
		d.recordOutcome(OutcomeIgnored, "", "")
		return nil
	}

	return d.enqueueTargets(ctx, event, targets, nil)
}

// DispatchToPlugin delivers an event to one specific registered plugin.
func (d *Dispatcher) DispatchToPlugin(ctx context.Context, pluginID string, event chatevent.Event) DeliveryResult {
	return d.dispatchOne(ctx, pluginID, event, nil)
}

// DispatchScheduledEvent keeps run bookkeeping outside the event sent to a plugin.
func (d *Dispatcher) DispatchScheduledEvent(ctx context.Context, pluginID string, event chatevent.Event, run scheduler.RunContext) DeliveryResult {
	return d.dispatchOne(ctx, pluginID, event, &run)
}

func (d *Dispatcher) dispatchOne(ctx context.Context, pluginID string, event chatevent.Event, run *scheduler.RunContext) DeliveryResult {
	results := d.enqueueTargets(ctx, event, []string{pluginID}, run)
	if len(results) == 0 {
		return DeliveryResult{
			PluginID:  pluginID,
			Outcome:   OutcomeError,
			ErrorCode: "platform.invalid_request",
		}
	}
	return results[0]
}
func (d *Dispatcher) enqueueTargets(ctx context.Context, event chatevent.Event, targets []string, run *scheduler.RunContext) []DeliveryResult {
	results := make([]DeliveryResult, 0, len(targets))
	for _, pluginID := range targets {
		if ctx.Err() != nil {
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeError, ErrorCode: "plugin.event_canceled"})
			d.recordOutcome(OutcomeDropped, pluginID, "event_canceled")
			continue
		}
		d.mu.RLock()
		slot, ok := d.slots[pluginID]
		deliverable := ok && slotIsDeliverable(slot)
		if !ok || !deliverable {
			d.mu.RUnlock()
			results = append(results, DeliveryResult{
				PluginID:  pluginID,
				Outcome:   OutcomeError,
				ErrorCode: "platform.invalid_request",
			})
			d.recordOutcome(OutcomeDropped, pluginID, "plugin_not_running")
			continue
		}

		control := isControlEvent(event.EventType)
		var eventCtx context.Context = deliveryContext{Context: slot.ctx, values: ctx}
		if event.EventType == "management.action" {
			eventCtx = ctx
		}
		item := dispatchItem{ctx: eventCtx, event: event, control: control, run: run}
		accepted := slot.tryEnqueue(item)
		d.mu.RUnlock()
		if accepted {
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDelivered})
			d.recordOutcome(OutcomeDelivered, pluginID, "")
		} else {
			if !slot.isAccepting() {
				results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeError, ErrorCode: errorcodes.PluginStopping})
				d.recordOutcome(OutcomeDropped, pluginID, "plugin_stopping")
				continue
			}
			reason := "queue_full"
			if control {
				reason = "control_queue_full"
			}
			d.logger.Warn("插件 "+pluginID+" 待处理任务过多，本次请求已丢弃。",
				"component", "dispatch",
				"plugin_id", pluginID,
				"event_id", event.EventID,
				"event_type", event.EventType,
				"reason", reason,
			)
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDropped, ErrorCode: errorcodes.PlatformRateLimited})
			d.recordOutcome(OutcomeDropped, pluginID, reason)
		}
	}
	return results
}

func isControlEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "plugin.started", "config.changed", "bot.identities.changed", "management.action":
		return true
	default:
		return false
	}
}

// selectTargets picks which plugins should receive the event.
// Must be called with d.mu held for reading.
func (d *Dispatcher) selectTargets(event chatevent.Event, commandName string) []string {
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
func slotDeclaresCommand(slot *pluginSlot, name string) bool {
	for _, cmd := range slot.commands {
		if cmd.Matches(name) {
			return true
		}
	}
	return false
}

func slotIsDeliverable(slot *pluginSlot) bool {
	if slot == nil || slot.runtime == nil {
		return false
	}
	return slot.runtime.ReadyForEvents()
}
func slotAcceptsEvent(slot *pluginSlot, eventType string) bool {
	// An empty manifest subscription list receives no ordinary fan-out.
	if len(slot.subscriptions) == 0 {
		return false
	}
	for _, sub := range slot.subscriptions {
		if sub == eventType || sub == "*" {
			return true
		}
	}
	return false
}
