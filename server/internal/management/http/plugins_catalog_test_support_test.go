package managementhttp

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"sort"
	"sync"
)

type testCatalog struct {
	mu    sync.RWMutex
	order []string
	items map[string]plugins.Snapshot
}

func newTestCatalog(entries []plugins.Snapshot) *testCatalog {
	catalog := &testCatalog{}
	catalog.Replace(entries)
	return catalog
}

func (c *testCatalog) List() []plugins.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]plugins.Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, plugins.CloneSnapshot(c.items[pluginID]))
	}
	return result
}

func (c *testCatalog) Get(pluginID string) (plugins.Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot, ok := c.items[pluginID]
	if !ok {
		return plugins.Snapshot{}, false
	}
	return plugins.CloneSnapshot(snapshot), true
}

func (c *testCatalog) Replace(entries []plugins.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	items := make(map[string]plugins.Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		items[entry.PluginID] = plugins.CloneSnapshot(entry)
		if _, ok := seen[entry.PluginID]; ok {
			continue
		}
		seen[entry.PluginID] = struct{}{}
		order = append(order, entry.PluginID)
	}
	sort.Strings(order)
	c.items = items
	c.order = order
}

func (c *testCatalog) SetDesiredState(pluginID string, desired string) (plugins.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if entry.RegistrationState != "installed" || entry.DesiredState == desired {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}
	entry.DesiredState = desired
	entry.DisplayState = plugins.DefaultDisplayState(entry)
	c.items[pluginID] = entry
	return plugins.CloneSnapshot(entry), nil
}
