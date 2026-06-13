package plugins

import (
	"sort"
)

func NewCatalog(entries []Snapshot) *Catalog {
	items := make(map[string]Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		items[entry.PluginID] = cloneSnapshot(entry)
		if _, ok := seen[entry.PluginID]; ok {
			continue
		}
		seen[entry.PluginID] = struct{}{}
		order = append(order, entry.PluginID)
	}

	sort.Strings(order)

	return &Catalog{
		order:       order,
		items:       items,
		subscribers: make(map[uint64]chan Snapshot),
	}
}

func (c *Catalog) List() []Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, cloneSnapshot(c.items[pluginID]))
	}

	return result
}

func (c *Catalog) Get(pluginID string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, false
	}

	return cloneSnapshot(entry), true
}

func (c *Catalog) Replace(entries []Snapshot) {
	c.mu.Lock()

	items := make(map[string]Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	updated := make([]Snapshot, 0, len(entries))

	for _, entry := range entries {
		cloned := cloneSnapshot(entry)
		items[entry.PluginID] = cloned
		if current, ok := c.items[entry.PluginID]; !ok || pluginStateChanged(current, cloned) {
			updated = append(updated, cloned)
		}
		if _, ok := seen[entry.PluginID]; ok {
			continue
		}
		seen[entry.PluginID] = struct{}{}
		order = append(order, entry.PluginID)
	}

	sort.Strings(order)
	c.items = items
	c.order = order
	c.mu.Unlock()

	c.publishMany(updated)
}
