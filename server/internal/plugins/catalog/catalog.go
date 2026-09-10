package catalog

import (
	"sort"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type Catalog struct {
	mu    sync.RWMutex
	order []string
	items map[string]plugins.Snapshot
	hub   pubsub.Hub[plugins.Snapshot]
}

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
		order: order,
		items: items,
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

func (c *Catalog) RefreshCommands(pluginID string, settings map[string]any) (plugins.Snapshot, bool) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return plugins.Snapshot{}, false
	}

	current := entry
	entry.Commands = ProjectCommands(entry, settings)
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

func (c *Catalog) Replace(entries []plugins.Snapshot) {
	c.replace(entries, "")
}

// RefreshInstalled applies one installation result without resetting unrelated runtimes.
func (c *Catalog) RefreshInstalled(entries []plugins.Snapshot, pluginID string) {
	c.replace(entries, pluginID)
}

func (c *Catalog) replace(entries []plugins.Snapshot, installedID string) {
	c.mu.Lock()
	if installedID != "" {
		selected := make([]plugins.Snapshot, 0, len(c.items)+1)
		for _, current := range c.items {
			if current.PluginID != installedID {
				selected = append(selected, current)
			}
		}
		for _, entry := range entries {
			if entry.PluginID != installedID {
				continue
			}
			if current, ok := c.items[installedID]; ok {
				entry.DesiredState = current.DesiredState
				entry.RuntimeState = current.RuntimeState
				entry.RuntimeErrorCode = current.RuntimeErrorCode
				entry.RuntimeErrorMessage = current.RuntimeErrorMessage
				entry.DeadLetter = current.DeadLetter
			}
			selected = append(selected, entry)
		}
		entries = selected
	}

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
