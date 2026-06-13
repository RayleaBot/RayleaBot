package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) SavePackageMetadata(ctx context.Context, pkg PackageMetadata) error {
	if err := r.writeQ.SavePackageMetadata(ctx, sqlcgen.SavePackageMetadataParams{
		PluginID:     pkg.PluginID,
		SourceType:   pkg.SourceType,
		SourceRef:    pkg.SourceRef,
		Version:      pkg.Version,
		ManifestHash: pkg.ManifestHash,
		PackageHash:  pkg.PackageHash,
		InstalledAt:  pkg.InstalledAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("upsert plugin package metadata for %s: %w", pkg.PluginID, err)
	}
	return nil
}

func (r *SQLiteRepository) DeletePackageMetadata(ctx context.Context, pluginID string) error {
	if err := r.writeQ.DeletePackageMetadata(ctx, pluginID); err != nil {
		return fmt.Errorf("delete plugin package metadata for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteRepository) LoadAllPackageMetadata(ctx context.Context) (map[string]PackageMetadata, error) {
	rows, err := r.readQ.LoadAllPackageMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("query plugin package metadata: %w", err)
	}

	metadata := make(map[string]PackageMetadata, len(rows))
	for _, row := range rows {
		installedAt, _ := time.Parse(time.RFC3339Nano, row.InstalledAt)
		metadata[row.PluginID] = PackageMetadata{
			PluginID:     row.PluginID,
			SourceType:   row.SourceType,
			SourceRef:    row.SourceRef,
			Version:      row.Version,
			ManifestHash: row.ManifestHash,
			PackageHash:  row.PackageHash,
			InstalledAt:  installedAt,
		}
	}
	return metadata, nil
}
