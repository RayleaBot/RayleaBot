package plugins

func (c *Catalog) RefreshCommands(pluginID string, settings map[string]any) (Snapshot, bool) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, false
	}

	current := entry
	entry.Commands = ProjectCommands(entry, settings)
	changed := pluginStateChanged(current, entry)
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	published := []Snapshot{updated}
	if changed {
		published = make([]Snapshot, 0, len(c.order))
		for _, id := range c.order {
			published = append(published, cloneSnapshot(c.items[id]))
		}
	}
	c.mu.Unlock()

	if changed {
		c.publishMany(published)
	}
	return updated, true
}
