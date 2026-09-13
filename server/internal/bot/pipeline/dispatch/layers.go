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
	ids := d.selectTargets(event, command)
	result := make([]messageCandidate, 0, len(ids))
	directed := false
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		slot := d.slots[id]
		selected[id] = true
		directed = directed || command != "" && slotDeclaresCommand(slot, command)
		result = append(result, messageCandidate{id: id, slot: slot, policy: slot.messagePolicy})
	}
	if directed {
		for id, slot := range d.slots {
			if !selected[id] && slotIsDeliverable(slot) && slot.messagePolicy.Priority > 0 && slotAcceptsEvent(slot, event.EventType) {
				result = append(result, messageCandidate{id: id, slot: slot, policy: slot.messagePolicy, beforeCommand: true})
			}
		}
	}
	return result
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
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].beforeCommand != candidates[j].beforeCommand {
			return candidates[i].beforeCommand
		}
		if candidates[i].policy.Priority != candidates[j].policy.Priority {
			return candidates[i].policy.Priority > candidates[j].policy.Priority
		}
		return candidates[i].id < candidates[j].id
	})
	if len(candidates) == 0 {
		d.recordOutcome(OutcomeIgnored, "", "")
		return nil
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
		result := d.enqueueTarget(ctx, event, candidate.id, nil, &enqueueOptions{expected: candidate.slot, gate: layer.gate})
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
