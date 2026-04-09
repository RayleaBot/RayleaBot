package dispatch

import (
	"context"
	"log/slog"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

// runtimeDeliverer is the interface a plugin runtime must satisfy for dispatch.
type runtimeDeliverer interface {
	DeliverEvent(context.Context, runtime.Event) (runtime.Delivery, error)
	Snapshot() runtime.Snapshot
}

// Outcome represents the result of delivering an event to a single plugin.
type Outcome string

const (
	OutcomeDelivered Outcome = "delivered"
	OutcomeError     Outcome = "error"
	OutcomeDropped   Outcome = "dropped"
)

// DeliveryResult records the outcome of event delivery to a single plugin.
type DeliveryResult struct {
	PluginID  string
	Outcome   Outcome
	ErrorCode string
}

// CommandDecl captures a plugin's declared command for directed delivery.
type CommandDecl struct {
	Name       string
	Aliases    []string
	Permission string
}

type dispatchItem struct {
	ctx   context.Context
	event runtime.Event
}

type pluginSlot struct {
	runtime       runtimeDeliverer
	subscriptions []string
	commands      []CommandDecl
	queue         chan dispatchItem
	done          chan struct{}
}

// Dispatcher manages per-plugin event queues and fan-out delivery.
type Dispatcher struct {
	logger    *slog.Logger
	sender    outbound.ActionSender
	resolver  outbound.ReplyTargetResolver
	queueSize int
	mu        sync.RWMutex
	slots     map[string]*pluginSlot
}

// New creates a Dispatcher.
func New(logger *slog.Logger, sender outbound.ActionSender, resolver outbound.ReplyTargetResolver, queueSize int) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if queueSize <= 0 {
		queueSize = 16
	}
	return &Dispatcher{
		logger:    logger,
		sender:    sender,
		resolver:  resolver,
		queueSize: queueSize,
		slots:     make(map[string]*pluginSlot),
	}
}

// Register adds a plugin runtime to the dispatch registry and starts its
// delivery worker goroutine. The rt parameter must implement DeliverEvent
// and Snapshot (both *runtime.Manager and test fakes satisfy this).
func (d *Dispatcher) Register(pluginID string, rt runtimeDeliverer, subs []string, cmds []CommandDecl) {
	d.mu.Lock()
	old, replacing := d.slots[pluginID]
	if replacing {
		delete(d.slots, pluginID)
	}

	slot := &pluginSlot{
		runtime:       rt,
		subscriptions: append([]string(nil), subs...),
		commands:      append([]CommandDecl(nil), cmds...),
		queue:         make(chan dispatchItem, d.queueSize),
		done:          make(chan struct{}),
	}
	d.slots[pluginID] = slot
	go d.worker(pluginID, slot)
	d.mu.Unlock()

	if replacing {
		close(old.queue)
		<-old.done
	}
}

// Deregister removes a plugin from dispatch and stops its worker.
func (d *Dispatcher) Deregister(pluginID string) {
	d.mu.Lock()
	slot, ok := d.slots[pluginID]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.slots, pluginID)
	d.mu.Unlock()

	close(slot.queue)
	<-slot.done
}

// PluginIDs returns a snapshot of currently registered plugin IDs.
func (d *Dispatcher) PluginIDs() []string {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	ids := make([]string, 0, len(d.slots))
	for id := range d.slots {
		ids = append(ids, id)
	}
	return ids
}

// HasPlugin reports whether a plugin slot is currently registered.
func (d *Dispatcher) HasPlugin(pluginID string) bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok := d.slots[pluginID]
	return ok
}

// HasDeliverablePlugins reports whether at least one registered runtime is in
// the running state and can accept delivery.
func (d *Dispatcher) HasDeliverablePlugins() bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, slot := range d.slots {
		if slotIsDeliverable(slot) {
			return true
		}
	}
	return false
}

// HasDeliverablePlugin reports whether the given plugin currently has a
// running runtime and can accept delivery.
func (d *Dispatcher) HasDeliverablePlugin(pluginID string) bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	return slotIsDeliverable(slot)
}

// Dispatch fans out an event to all matching registered plugins.
// If commandName is non-empty, plugins declaring that command are
// preferred (directed delivery). Otherwise all message-subscribed
// plugins receive the event.
func (d *Dispatcher) Dispatch(ctx context.Context, event runtime.Event, commandName string) []DeliveryResult {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	targets := d.selectTargets(event, commandName)
	d.mu.RUnlock()

	return d.enqueueTargets(ctx, event, targets)
}

// DispatchToPlugin delivers an event to one specific registered plugin.
func (d *Dispatcher) DispatchToPlugin(ctx context.Context, pluginID string, event runtime.Event) DeliveryResult {
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

func (d *Dispatcher) enqueueTargets(ctx context.Context, event runtime.Event, targets []string) []DeliveryResult {
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
			continue
		}

		item := dispatchItem{ctx: ctx, event: event}
		select {
		case slot.queue <- item:
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDelivered})
		default:
			d.logger.Warn("dispatch queue full, dropping event",
				"component", "dispatch",
				"plugin_id", pluginID,
				"event_id", event.EventID,
			)
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDropped})
		}
	}
	return results
}

// selectTargets picks which plugins should receive the event.
// Must be called with d.mu held for reading.
func (d *Dispatcher) selectTargets(event runtime.Event, commandName string) []string {
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
	return slot.runtime.Snapshot().State == runtime.StateRunning
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

// worker is the per-plugin goroutine that sequentially delivers events.
func (d *Dispatcher) worker(pluginID string, slot *pluginSlot) {
	defer close(slot.done)

	for item := range slot.queue {
		delivery, err := slot.runtime.DeliverEvent(item.ctx, item.event)
		if err != nil {
			d.logger.Warn("dispatch delivery failed",
				"component", "dispatch",
				"plugin_id", pluginID,
				"event_id", item.event.EventID,
				"err", err.Error(),
			)
			continue
		}

		if delivery.Action != nil {
			d.executeAction(item.ctx, pluginID, item.event, *delivery.Action)
		}
	}
}

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, event runtime.Event, action runtime.Action) {
	if d.sender == nil {
		return
	}

	_, err := outbound.SendAction(ctx, d.sender, d.resolver, event, action)

	if err != nil {
		d.logger.Warn("outbound action failed",
			"component", "dispatch",
			"plugin_id", pluginID,
			"action_kind", action.Kind,
			"err", err.Error(),
		)
	}
}

// Close deregisters all plugins and stops all workers.
func (d *Dispatcher) Close() {
	d.mu.Lock()
	slots := make(map[string]*pluginSlot, len(d.slots))
	for id, slot := range d.slots {
		slots[id] = slot
	}
	d.slots = make(map[string]*pluginSlot)
	d.mu.Unlock()

	for _, slot := range slots {
		close(slot.queue)
		<-slot.done
	}
}
