package dispatch

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

// Register adds a plugin runtime to the dispatch registry and starts its
// delivery worker goroutine. The rt parameter must implement DeliverEvent
// and Snapshot (both *runtime.Manager and test fakes satisfy this).
func (d *Dispatcher) Register(pluginID string, rt runtimeDeliverer, subs []string, cmds []plugins.Command, concurrency int) bool {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return false
	}
	old, replacing := d.slots[pluginID]
	if replacing {
		delete(d.slots, pluginID)
		d.retireSlotLocked(old)
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	slot := d.newPluginSlot(rt, subs, cmds, concurrency)
	d.slots[pluginID] = slot
	go d.worker(pluginID, slot)
	d.mu.Unlock()

	if replacing {
		old.closeQueues()
		old.cancel()
	}
	return true
}

func (d *Dispatcher) IsClosed() bool { d.mu.RLock(); defer d.mu.RUnlock(); return d.closed }

func (d *Dispatcher) retireSlotLocked(slot *pluginSlot) {
	d.retired[slot] = struct{}{}
	go func() { <-slot.done; d.mu.Lock(); delete(d.retired, slot); d.mu.Unlock() }()
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

	slot.closeQueues()
	slot.cancel()
	<-slot.done
}

// PluginIDs returns a snapshot of currently registered plugin IDs.
func (d *Dispatcher) PluginIDs() []string {

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

	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok := d.slots[pluginID]
	return ok
}
func (d *Dispatcher) UpdateCommands(pluginID string, cmds []plugins.Command) bool {

	d.mu.Lock()
	defer d.mu.Unlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	slot.commands = plugins.CloneCommands(cmds)
	return true
}

// HasDeliverablePlugins reports whether at least one registered runtime is in
// the running state and can accept delivery.
func (d *Dispatcher) HasDeliverablePlugins() bool {

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

	d.mu.RLock()
	defer d.mu.RUnlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	return slotIsDeliverable(slot)
}

// Close deregisters all plugins and stops all workers.
func (d *Dispatcher) Close() {
	d.flushMu.Lock()
	stop := d.flushStop
	done := d.flushDone
	d.flushStop = nil
	d.flushDone = nil
	d.flushMu.Unlock()
	if stop != nil {
		close(stop)
		if done != nil {
			<-done
		}
	}

	d.mu.Lock()
	d.closed = true
	slots := make(map[*pluginSlot]struct{}, len(d.slots)+len(d.retired))
	for _, slot := range d.slots {
		slots[slot] = struct{}{}
	}
	for slot := range d.retired {
		slots[slot] = struct{}{}
	}
	d.slots = make(map[string]*pluginSlot)
	d.retired = make(map[*pluginSlot]struct{})
	d.mu.Unlock()

	for slot := range slots {
		slot.closeQueues()
		slot.cancel()
	}
	for slot := range slots {
		<-slot.done
	}
}

func (d *Dispatcher) newPluginSlot(rt runtimeDeliverer, subs []string, cmds []plugins.Command, concurrency int) *pluginSlot {
	if concurrency <= 0 {
		concurrency = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &pluginSlot{
		ctx:           ctx,
		cancel:        cancel,
		runtime:       rt,
		subscriptions: append([]string(nil), subs...),
		commands:      plugins.CloneCommands(cmds),
		concurrency:   concurrency,
		eventQueue:    make(chan dispatchItem, d.queueSize),
		controlQueue:  make(chan dispatchItem, d.controlQueueSize),
		done:          make(chan struct{}),
		accepting:     true,
		eventLimit:    d.queueSize,
		controlLimit:  d.controlQueueSize,
	}
}

func (s *pluginSlot) tryEnqueue(item dispatchItem) bool {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if !s.accepting {
		return false
	}
	if item.control {
		if s.pendingControl >= s.controlLimit {
			return false
		}
		s.pendingControl++
		s.controlQueue <- item
		return true
	}
	if s.pendingEvents >= s.eventLimit {
		return false
	}
	s.pendingEvents++
	s.eventQueue <- item
	return true
}

func (s *pluginSlot) isAccepting() bool {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	return s.accepting
}

func (s *pluginSlot) markStarted(item dispatchItem) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if item.control {
		if s.pendingControl > 0 {
			s.pendingControl--
		}
		return
	}
	if s.pendingEvents > 0 {
		s.pendingEvents--
	}
}

func (s *pluginSlot) closeQueues() {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if !s.accepting {
		return
	}
	s.accepting = false
	close(s.controlQueue)
	close(s.eventQueue)
}
