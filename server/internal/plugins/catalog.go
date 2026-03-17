package plugins

import "sort"

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
	result := make([]Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, cloneSnapshot(c.items[pluginID]))
	}

	return result
}

func (c *Catalog) Get(pluginID string) (Snapshot, bool) {
	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, false
	}

	return cloneSnapshot(entry), true
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	return cloned
}
