package plugins

func ApplyPackageMetadata(entries []Snapshot, metadata map[string]PackageMetadata) []Snapshot {
	if len(entries) == 0 {
		return nil
	}

	enriched := make([]Snapshot, 0, len(entries))
	for _, entry := range entries {
		cloned := cloneSnapshot(entry)
		if pkg, ok := metadata[cloned.PluginID]; ok {
			cloned.PackageSourceType = pkg.SourceType
			cloned.PackageSourceRef = pkg.SourceRef
		}
		enriched = append(enriched, cloned)
	}
	return enriched
}
