package plugins

// ReadDisplaySnapshots reads caller-owned fields used by management summaries,
// details and plugin.list. Catalog implementations without a display projection
// retain their existing List behavior.
func ReadDisplaySnapshots(catalog interface{ List() []Snapshot }) []Snapshot {
	if view, ok := catalog.(interface{ DisplaySnapshots() []Snapshot }); ok {
		return view.DisplaySnapshots()
	}
	return catalog.List()
}

func ReadDisplaySnapshot(catalog interface{ Get(string) (Snapshot, bool) }, pluginID string) (Snapshot, bool) {
	if view, ok := catalog.(interface{ DisplaySnapshot(string) (Snapshot, bool) }); ok {
		return view.DisplaySnapshot(pluginID)
	}
	return catalog.Get(pluginID)
}

// CloneDisplaySnapshot omits declarations used only by installation and runtime
// assembly while preserving independent ownership of all display fields.
func CloneDisplaySnapshot(snapshot Snapshot) Snapshot {
	snapshot.DefaultConfig = nil
	snapshot.Services = nil
	snapshot.ManifestCommands = nil
	snapshot.ManifestCommandPrefixes = nil
	snapshot.RenderTemplates = nil
	return CloneSnapshot(snapshot)
}

type CatalogStateCounts struct {
	Total   int
	Running int
	Failed  int
}

func (counts *CatalogStateCounts) Add(snapshot Snapshot) {
	counts.Total++
	state, _ := ProjectState(snapshot)
	switch state {
	case PluginStateRunning:
		counts.Running++
	case PluginStateFailed:
		counts.Failed++
	}
}

func ReadCatalogStateCounts(catalog interface{ List() []Snapshot }) CatalogStateCounts {
	if view, ok := catalog.(interface{ StateCounts() CatalogStateCounts }); ok {
		return view.StateCounts()
	}
	var counts CatalogStateCounts
	for _, snapshot := range catalog.List() {
		counts.Add(snapshot)
	}
	return counts
}

func ReadCatalogCount(catalog interface{ List() []Snapshot }) int {
	if view, ok := catalog.(interface{ Count() int }); ok {
		return view.Count()
	}
	return len(catalog.List())
}
