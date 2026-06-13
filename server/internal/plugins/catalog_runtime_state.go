package plugins

func (c *Catalog) SetRuntimeState(pluginID string, runtimeState string) (Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, ErrPluginNotFound
	}

	current := entry
	entry.RuntimeState = runtimeState
	if runtimeState != "dead_letter" {
		entry.DeadLetter = nil
	}
	entry.DisplayState = defaultDisplayState(entry)
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	c.mu.Unlock()

	if pluginStateChanged(current, entry) {
		c.publish(updated)
	}
	return updated, nil
}
