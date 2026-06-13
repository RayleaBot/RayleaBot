package deps

import (
	"errors"
	"path/filepath"
)

func (m *Manager) Inspect(kind string) (*BootstrapInspection, error) {
	if m == nil {
		return nil, errors.New("deps manager is required")
	}

	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		return nil, classifyBootstrapError(m.repoRoot, kind, nil, "manifest", "", nil, err)
	}
	inspection := &BootstrapInspection{
		Kind:             kind,
		Resource:         resource,
		ArchivePath:      filepath.Join(CacheRoot(m.repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat)),
		StoreRoot:        StoreRoot(m.repoRoot, resource),
		MetadataComplete: manifest.HasPlatform(CurrentPlatform()) && ResourceMetadataComplete(resource),
	}
	if inspection.MetadataComplete && verifyFileSHA256(inspection.ArchivePath, resource.SHA256) == nil {
		inspection.CachedArchivePresent = true
	}
	if _, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		inspection.PreparedStorePresent = true
	}
	return inspection, nil
}
