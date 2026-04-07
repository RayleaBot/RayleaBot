package plugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rayleabot/server/internal/sqlcgen"
	"rayleabot/server/internal/storage"
)

type DesiredStateRepository interface {
	LoadDesiredStates(context.Context) (map[string]string, error)
	SaveDesiredState(context.Context, string, string, time.Time) error
	DeleteDesiredState(context.Context, string) error
}

type PackageMetadata struct {
	PluginID     string
	SourceType   string
	SourceRef    string
	Version      string
	ManifestHash string
	PackageHash  string
	InstalledAt  time.Time
}

type PackageRepository interface {
	SavePackageMetadata(context.Context, PackageMetadata) error
	DeletePackageMetadata(context.Context, string) error
}

type PackageMetadataLoader interface {
	LoadAllPackageMetadata(context.Context) (map[string]PackageMetadata, error)
}

type PluginGrant struct {
	PluginID   string
	Capability string
	ScopeJSON  string // JSON-encoded scope boundaries (http_hosts, storage_roots, etc.)
	GrantedAt  time.Time
	ExpiresAt  *time.Time
}

type GrantRepository interface {
	LoadGrants(ctx context.Context, pluginID string) ([]PluginGrant, error)
	LoadAllGrants(ctx context.Context) (map[string][]string, error)
	SaveGrant(ctx context.Context, grant PluginGrant) error
	DeleteGrant(ctx context.Context, pluginID, capability string) error
	DeleteAllGrants(ctx context.Context, pluginID string) error
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
	}, nil
}

func (r *SQLiteRepository) LoadDesiredStates(ctx context.Context) (map[string]string, error) {
	rows, err := r.readQ.LoadDesiredStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("query plugin desired_state rows: %w", err)
	}

	states := make(map[string]string, len(rows))
	for _, row := range rows {
		states[row.PluginID] = row.DesiredState
	}
	return states, nil
}

func (r *SQLiteRepository) SaveDesiredState(ctx context.Context, pluginID string, desiredState string, updatedAt time.Time) error {
	if err := r.writeQ.SaveDesiredState(ctx, sqlcgen.SaveDesiredStateParams{
		PluginID:     pluginID,
		DesiredState: desiredState,
		UpdatedAt:    updatedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("upsert plugin desired_state for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteDesiredState(ctx context.Context, pluginID string) error {
	if err := r.writeQ.DeleteDesiredState(ctx, pluginID); err != nil {
		return fmt.Errorf("delete plugin desired_state for %s: %w", pluginID, err)
	}
	return nil
}

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

func (r *SQLiteRepository) LoadGrants(ctx context.Context, pluginID string) ([]PluginGrant, error) {
	rows, err := r.readQ.LoadGrants(ctx, pluginID)
	if err != nil {
		return nil, fmt.Errorf("query grants for %s: %w", pluginID, err)
	}

	now := time.Now().UTC()
	var grants []PluginGrant
	for _, row := range rows {
		g := PluginGrant{
			PluginID:   row.PluginID,
			Capability: row.Capability,
			ScopeJSON:  row.ScopeJson,
		}
		g.GrantedAt, _ = time.Parse(time.RFC3339Nano, row.GrantedAt)
		if parsed, ok := parseGrantExpiry(row.ExpiresAt); ok {
			g.ExpiresAt = parsed
			if !parsed.After(now) {
				continue
			}
		}
		grants = append(grants, g)
	}
	return grants, nil
}

func (r *SQLiteRepository) LoadAllGrants(ctx context.Context) (map[string][]string, error) {
	rows, err := r.readQ.LoadAllGrants(ctx)
	if err != nil {
		return nil, fmt.Errorf("query all grants: %w", err)
	}

	now := time.Now().UTC()
	grants := make(map[string][]string)
	for _, row := range rows {
		if parsed, ok := parseGrantExpiry(row.ExpiresAt); ok && !parsed.After(now) {
			continue
		}
		grants[row.PluginID] = append(grants[row.PluginID], row.Capability)
	}
	return grants, nil
}

func (r *SQLiteRepository) SaveGrant(ctx context.Context, grant PluginGrant) error {
	if err := r.writeQ.SaveGrant(ctx, sqlcgen.SaveGrantParams{
		PluginID:   grant.PluginID,
		Capability: grant.Capability,
		ScopeJson:  grant.ScopeJSON,
		GrantedAt:  grant.GrantedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt:  formatGrantExpiryNullString(grant.ExpiresAt),
	}); err != nil {
		return fmt.Errorf("upsert grant for %s/%s: %w", grant.PluginID, grant.Capability, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteGrant(ctx context.Context, pluginID, capability string) error {
	if err := r.writeQ.DeleteGrant(ctx, sqlcgen.DeleteGrantParams{
		PluginID:   pluginID,
		Capability: capability,
	}); err != nil {
		return fmt.Errorf("delete grant %s/%s: %w", pluginID, capability, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteAllGrants(ctx context.Context, pluginID string) error {
	if err := r.writeQ.DeleteAllGrants(ctx, pluginID); err != nil {
		return fmt.Errorf("delete all grants for %s: %w", pluginID, err)
	}
	return nil
}

func formatGrantExpiryNullString(expiresAt *time.Time) sql.NullString {
	if expiresAt == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: expiresAt.UTC().Format(time.RFC3339Nano), Valid: true}
}

func parseGrantExpiry(value sql.NullString) (*time.Time, bool) {
	if !value.Valid || value.String == "" {
		return nil, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}
