package plugincatalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) SetRuntimeState(pluginID string, runtimeState string) (plugins.Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}

	current := entry
	entry.RuntimeState = runtimeState
	if runtimeState != "dead_letter" {
		entry.DeadLetter = nil
	}
	entry.DisplayState = plugins.DefaultDisplayState(entry)
	c.items[pluginID] = entry
	updated := plugins.CloneSnapshot(entry)
	c.mu.Unlock()

	if pluginStateChanged(current, entry) {
		c.publish(updated)
	}
	return updated, nil
}
