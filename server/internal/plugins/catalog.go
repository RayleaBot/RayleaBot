package plugins

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrPluginNotFound = errors.New("plugin not found")
	ErrStateConflict  = errors.New("state conflict")
)

type Snapshot struct {
	PluginID          string
	Name              string
	Version           string
	Runtime           string
	Entry             string
	Description       string
	ManifestPath      string
	SourceRoot        string
	SourceRoots       []string
	Valid             bool
	ValidationSummary string
	RegistrationState string
	DesiredState      string
	RuntimeState      string
	DisplayState      string
	ConflictPaths     []string
}

type Catalog struct {
	mu    sync.RWMutex
	order []string
	items map[string]Snapshot
}

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
		order: order,
		items: items,
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

func (c *Catalog) SetDesiredState(pluginID string, desired string) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, ErrPluginNotFound
	}

	if entry.RegistrationState != "installed" {
		return Snapshot{}, ErrStateConflict
	}

	if entry.DesiredState == desired {
		return Snapshot{}, ErrStateConflict
	}

	entry.DesiredState = desired
	c.items[pluginID] = entry

	return cloneSnapshot(entry), nil
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	return cloned
}
