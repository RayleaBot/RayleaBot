package dispatch

import (
	"context"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

type messageCandidate struct {
	id            string
	slot          *pluginSlot
	policy        MessagePolicy
	beforeCommand bool
}
type layerDelivery struct {
	result DeliveryResult
	policy MessagePolicy
}
type messageLayer struct {
	gate       *layerGate
	deliveries []layerDelivery
}

// Caller holds the registry read lock. Policy and generation are immutable for
// this admission even when the next message observes a reloaded manifest.
func (d *Dispatcher) messageCandidates(event chatevent.Event, command string) []messageCandidate {
	ids := d.commandTargets(event, command)
	routes, known := d.messageRoutes[event.EventType]
	if !known {
		routes = d.messageWildcards
	}
	if len(ids) == 0 {
		if event.CommandResolved && len(event.CommandTargets) > 0 {
			return nil
		}
		result := make([]messageCandidate, 0, len(routes))
		for _, candidate := range routes {
			if slotIsDeliverable(candidate.slot) {
				result = append(result, candidate)
			}
		}
		return result
	}
	selected := make(map[string]bool, len(ids))
	commands := make([]messageCandidate, 0, len(ids))
	for _, id := range ids {
		slot := d.slots[id]
		selected[id] = true
		commands = append(commands, messageCandidate{id: id, slot: slot, policy: slot.messagePolicy})
	}
	if len(commands) > 1 {
		sort.Slice(commands, func(i, j int) bool { return candidateLess(commands[i], commands[j]) })
	}
	var before []messageCandidate
	for _, candidate := range routes {
		if candidate.policy.Priority <= 0 {
			break
		}
		if !selected[candidate.id] && slotIsDeliverable(candidate.slot) {
			candidate.beforeCommand = true
			before = append(before, candidate)
		}
	}
	if len(before) == 0 {
		return commands
	}
	return append(before, commands...)
}

func candidateLess(left, right messageCandidate) bool {
	if left.policy.Priority != right.policy.Priority {
		return left.policy.Priority > right.policy.Priority
	}
	return left.id < right.id
}

// Subscription ordering changes with registry membership, not with individual
// messages. Readiness is still checked at admission and again before delivery.
// The caller holds mu for the whole replacement of these derived routes.
func (d *Dispatcher) rebuildMessageRoutesLocked() {
	ordered := make([]messageCandidate, 0, len(d.slots))
	routes := make(map[string][]messageCandidate)
	for id, slot := range d.slots {
		ordered = append(ordered, messageCandidate{id: id, slot: slot, policy: slot.messagePolicy})
		for _, subscription := range slot.subscriptions {
			if subscription != "*" {
				routes[subscription] = nil
			}
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return candidateLess(ordered[i], ordered[j]) })
	var wildcards []messageCandidate
	for _, candidate := range ordered {
		if slotAcceptsEvent(candidate.slot, "*") {
			wildcards = append(wildcards, candidate)
		}
		for eventType := range routes {
			if slotAcceptsEvent(candidate.slot, eventType) {
				routes[eventType] = append(routes[eventType], candidate)
			}
		}
	}
	d.messageRoutes, d.messageWildcards = routes, wildcards
}

func (d *Dispatcher) dispatchLayered(ctx context.Context, event chatevent.Event, command string) []DeliveryResult {
	d.admissionMu.Lock()
	defer d.admissionMu.Unlock()
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil
	}
	candidates := d.messageCandidates(event, command)
	d.mu.RUnlock()
	if len(candidates) == 0 {
		d.recordOutcome(OutcomeIgnored, "", "")
		return nil
	}
	first, last := candidates[0], candidates[len(candidates)-1]
	if first.policy.Priority == last.policy.Priority && first.beforeCommand == last.beforeCommand {
		results := make([]DeliveryResult, 0, len(candidates))
		for _, candidate := range candidates {
			results = append(results, d.enqueueTarget(ctx, eventForTarget(event, candidate.id), candidate.id, nil, &enqueueOptions{expected: candidate.slot}))
		}
		return results
	}
	var layers []messageLayer
	results := make([]DeliveryResult, 0, len(candidates))
	for i, candidate := range candidates {
		if i == 0 || candidate.policy.Priority != candidates[i-1].policy.Priority || candidate.beforeCommand != candidates[i-1].beforeCommand {
			var gate *layerGate
			if i > 0 {
				gate = &layerGate{done: make(chan struct{})}
			}
			layers = append(layers, messageLayer{gate: gate})
		}
		layer := &layers[len(layers)-1]
		result := d.enqueueTarget(ctx, eventForTarget(event, candidate.id), candidate.id, nil, &enqueueOptions{expected: candidate.slot, gate: layer.gate})
		results = append(results, result)
		layer.deliveries = append(layer.deliveries, layerDelivery{result: result, policy: candidate.policy})
	}
	if len(layers) > 1 {
		d.layersDone.Go(func() { advanceLayers(layers) })
	}
	return results
}

func advanceLayers(layers []messageLayer) {
	stopped := false
	for _, layer := range layers {
		if layer.gate != nil {
			layer.gate.skip = stopped
			close(layer.gate.done)
		}
		for _, item := range layer.deliveries {
			result, _ := item.result.Completion.Wait(context.Background())
			if result.Success && (result.Propagation == "stop" || result.Propagation == "" && item.policy.Block) {
				stopped = true
			}
		}
	}
}
