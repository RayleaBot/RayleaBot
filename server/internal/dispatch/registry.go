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

	slot := &pluginSlot{
		runtime:       rt,
		subscriptions: append([]string(nil), subs...),
		commands:      append([]CommandDecl(nil), cmds...),
		concurrency:   concurrency,
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
func (d *Dispatcher) UpdateCommands(pluginID string, cmds []CommandDecl) bool {
	if d == nil {
		return false
	}

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
		close(slot.queue)
		<-slot.done
	}
}
