package plugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}

func (r *SQLiteRepository) LoadDesiredStates(ctx context.Context) (map[string]string, error) {
	rows, err := r.read.QueryContext(ctx, `SELECT plugin_id, desired_state FROM plugin_instances`)
	if err != nil {
		return nil, fmt.Errorf("query plugin desired_state rows: %w", err)
	}
	defer rows.Close()

	states := make(map[string]string)
	for rows.Next() {
		var pluginID string
		var desiredState string
		if err := rows.Scan(&pluginID, &desiredState); err != nil {
			return nil, fmt.Errorf("scan plugin desired_state row: %w", err)
		}
		states[pluginID] = desiredState
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin desired_state rows: %w", err)
	}

	return states, nil
}

func (r *SQLiteRepository) SaveDesiredState(ctx context.Context, pluginID string, desiredState string, updatedAt time.Time) error {
	if _, err := r.write.ExecContext(
		ctx,
		`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(plugin_id) DO UPDATE SET
			desired_state = excluded.desired_state,
			updated_at = excluded.updated_at`,
		pluginID,
		desiredState,
		updatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert plugin desired_state for %s: %w", pluginID, err)
	}

	return nil
}

func (r *SQLiteRepository) DeleteDesiredState(ctx context.Context, pluginID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM plugin_instances WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("delete plugin desired_state for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteRepository) SavePackageMetadata(ctx context.Context, pkg PackageMetadata) error {
	if _, err := r.write.ExecContext(
		ctx,
		`INSERT INTO plugin_packages (
			plugin_id,
			source_type,
			source_ref,
			version,
			manifest_hash,
			package_hash,
			installed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plugin_id) DO UPDATE SET
			source_type = excluded.source_type,
			source_ref = excluded.source_ref,
			version = excluded.version,
			manifest_hash = excluded.manifest_hash,
			package_hash = excluded.package_hash,
			installed_at = excluded.installed_at`,
		pkg.PluginID,
		pkg.SourceType,
		pkg.SourceRef,
		pkg.Version,
		pkg.ManifestHash,
		pkg.PackageHash,
		pkg.InstalledAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert plugin package metadata for %s: %w", pkg.PluginID, err)
	}

	return nil
}

func (r *SQLiteRepository) DeletePackageMetadata(ctx context.Context, pluginID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM plugin_packages WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("delete plugin package metadata for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteRepository) LoadGrants(ctx context.Context, pluginID string) ([]PluginGrant, error) {
	rows, err := r.read.QueryContext(ctx, `SELECT plugin_id, capability, scope_json, granted_at, expires_at FROM plugin_grants WHERE plugin_id = ? ORDER BY capability`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("query grants for %s: %w", pluginID, err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var grants []PluginGrant
	for rows.Next() {
		var g PluginGrant
		var grantedAt string
		var expiresAt sql.NullString
		if err := rows.Scan(&g.PluginID, &g.Capability, &g.ScopeJSON, &grantedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan grant row: %w", err)
		}
		g.GrantedAt, _ = time.Parse(time.RFC3339Nano, grantedAt)
		if parsed, ok := parseGrantExpiry(expiresAt); ok {
			g.ExpiresAt = parsed
			if !parsed.After(now) {
				continue
			}
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (r *SQLiteRepository) LoadAllGrants(ctx context.Context) (map[string][]string, error) {
	rows, err := r.read.QueryContext(ctx, `SELECT plugin_id, capability, expires_at FROM plugin_grants ORDER BY plugin_id, capability`)
	if err != nil {
		return nil, fmt.Errorf("query all grants: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	grants := make(map[string][]string)
	for rows.Next() {
		var pluginID, capability string
		var expiresAt sql.NullString
		if err := rows.Scan(&pluginID, &capability, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan grant row: %w", err)
		}
		if parsed, ok := parseGrantExpiry(expiresAt); ok && !parsed.After(now) {
			continue
		}
		grants[pluginID] = append(grants[pluginID], capability)
	}
	return grants, rows.Err()
}

func (r *SQLiteRepository) SaveGrant(ctx context.Context, grant PluginGrant) error {
	if _, err := r.write.ExecContext(
		ctx,
		`INSERT INTO plugin_grants (plugin_id, capability, scope_json, granted_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(plugin_id, capability) DO UPDATE SET
			scope_json = excluded.scope_json,
			granted_at = excluded.granted_at,
			expires_at = excluded.expires_at`,
		grant.PluginID,
		grant.Capability,
		grant.ScopeJSON,
		grant.GrantedAt.UTC().Format(time.RFC3339Nano),
		formatGrantExpiry(grant.ExpiresAt),
	); err != nil {
		return fmt.Errorf("upsert grant for %s/%s: %w", grant.PluginID, grant.Capability, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteGrant(ctx context.Context, pluginID, capability string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM plugin_grants WHERE plugin_id = ? AND capability = ?`, pluginID, capability); err != nil {
		return fmt.Errorf("delete grant %s/%s: %w", pluginID, capability, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteAllGrants(ctx context.Context, pluginID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM plugin_grants WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("delete all grants for %s: %w", pluginID, err)
	}
	return nil
}

func formatGrantExpiry(expiresAt *time.Time) any {
	if expiresAt == nil {
		return nil
	}
	return expiresAt.UTC().Format(time.RFC3339Nano)
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
