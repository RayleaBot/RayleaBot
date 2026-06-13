package plugins

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

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
