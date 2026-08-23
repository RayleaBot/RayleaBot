package dispatch

// Register adds a plugin runtime to the dispatch registry and starts its
// delivery worker goroutine. The rt parameter must implement DeliverEvent
// and Snapshot (both *runtime.Manager and test fakes satisfy this).
func (d *Dispatcher) Register(pluginID string, rt runtimeDeliverer, subs []string, cmds []CommandDecl, concurrency int) {
	d.mu.Lock()
	old, replacing := d.slots[pluginID]
	if replacing {
		delete(d.slots, pluginID)
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

	slot.closeQueues()
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
func (d *Dispatcher) UpdateCommands(pluginID string, cmds []CommandDecl) bool {

	d.mu.Lock()
	defer d.mu.Unlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	slot.commands = append([]CommandDecl(nil), cmds...)
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
	slots := make(map[string]*pluginSlot, len(d.slots))
	for id, slot := range d.slots {
		slots[id] = slot
	}
	d.slots = make(map[string]*pluginSlot)
	d.mu.Unlock()

	for _, slot := range slots {
		slot.closeQueues()
		<-slot.done
	}
}

func (d *Dispatcher) newPluginSlot(rt runtimeDeliverer, subs []string, cmds []CommandDecl, concurrency int) *pluginSlot {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &pluginSlot{
		runtime:       rt,
		subscriptions: append([]string(nil), subs...),
		commands:      append([]CommandDecl(nil), cmds...),
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
