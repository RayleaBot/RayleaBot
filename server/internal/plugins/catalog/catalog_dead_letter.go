package plugincatalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) SetDeadLetterSnapshot(pluginID string, info plugins.DeadLetterSnapshot) (plugins.Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}

	current := entry
	copied := info
	entry.DeadLetter = &copied
	c.items[pluginID] = entry
	updated := plugins.CloneSnapshot(entry)
	c.mu.Unlock()

	if pluginStateChanged(current, entry) {
		c.publish(updated)
	}
	return updated, nil
}
