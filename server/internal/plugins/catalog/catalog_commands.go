package catalog

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginmanifest "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifest"
)

func (c *Catalog) RefreshCommands(pluginID string, settings map[string]any) (plugins.Snapshot, bool) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return plugins.Snapshot{}, false
	}

	current := entry
	entry.Commands = pluginmanifest.ProjectCommands(entry, settings)
	changed := pluginStateChanged(current, entry)
	c.items[pluginID] = entry
	updated := plugins.CloneSnapshot(entry)
	published := []plugins.Snapshot{updated}
	if changed {
		published = make([]plugins.Snapshot, 0, len(c.order))
		for _, id := range c.order {
			published = append(published, plugins.CloneSnapshot(c.items[id]))
		}
	}
	c.mu.Unlock()

	if changed {
		c.publishMany(published)
	}
	return updated, true
}
