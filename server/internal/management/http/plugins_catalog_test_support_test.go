package managementhttp

import (
	"sort"
	"sync"
)

type testCatalog struct {
	mu    sync.RWMutex
	order []string
	items map[string]Snapshot
}

func newTestCatalog(entries []Snapshot) *testCatalog {
	catalog := &testCatalog{}
	catalog.Replace(entries)
	return catalog
}

func (c *testCatalog) List() []Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, CloneSnapshot(c.items[pluginID]))
	}
	return result
}

func (c *testCatalog) Get(pluginID string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, false
	}
	return CloneSnapshot(snapshot), true
}

func (c *testCatalog) Replace(entries []Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	items := make(map[string]Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		items[entry.PluginID] = CloneSnapshot(entry)
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

func (c *testCatalog) SetDesiredState(pluginID string, desired string) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, ErrPluginNotFound
	}
	if entry.RegistrationState != "installed" || entry.DesiredState == desired {
		return Snapshot{}, ErrStateConflict
	}
	entry.DesiredState = desired
	entry.DisplayState = DefaultDisplayState(entry)
	c.items[pluginID] = entry
	return CloneSnapshot(entry), nil
}
