package plugins

func (c *Catalog) SetDeadLetterSnapshot(pluginID string, info DeadLetterSnapshot) (Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, ErrPluginNotFound
	}

	current := entry
	copied := info
	entry.DeadLetter = &copied
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	c.mu.Unlock()

	if pluginStateChanged(current, entry) {
		c.publish(updated)
	}
	return updated, nil
}
