package plugincatalog

import (
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func New(entries []plugins.Snapshot) *Catalog {
	items := make(map[string]plugins.Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		items[entry.PluginID] = plugins.CloneSnapshot(entry)
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
		subscribers: make(map[uint64]chan plugins.Snapshot),
	}
}

func (c *Catalog) List() []plugins.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]plugins.Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, plugins.CloneSnapshot(c.items[pluginID]))
	}

	return result
}

func (c *Catalog) Get(pluginID string) (plugins.Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return plugins.Snapshot{}, false
	}

	return plugins.CloneSnapshot(entry), true
}

func (c *Catalog) Replace(entries []plugins.Snapshot) {
	c.mu.Lock()

	items := make(map[string]plugins.Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	updated := make([]plugins.Snapshot, 0, len(entries))

	for _, entry := range entries {
		cloned := plugins.CloneSnapshot(entry)
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
