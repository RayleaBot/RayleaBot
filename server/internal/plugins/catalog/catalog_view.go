package catalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) DisplaySnapshots() []plugins.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]plugins.Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, plugins.CloneDisplaySnapshot(c.items[pluginID]))
	}
	return result
}

func (c *Catalog) DisplaySnapshot(pluginID string) (plugins.Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return plugins.Snapshot{}, false
	}
	return plugins.CloneDisplaySnapshot(entry), true
}

func (c *Catalog) StateCounts() plugins.CatalogStateCounts {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var counts plugins.CatalogStateCounts
	for _, pluginID := range c.order {
		counts.Add(c.items[pluginID])
	}
	return counts
}

func (c *Catalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.order)
}
