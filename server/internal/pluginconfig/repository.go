package pluginconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"rayleabot/server/internal/storage"
)

type Repository interface {
	SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error)
	Read(ctx context.Context, pluginID string, keys []string) (map[string]any, error)
	Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error)
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

func (r *SQLiteRepository) SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}

	namespace := namespaceForPlugin(pluginID)
	existing, err := r.namespaceCount(ctx, namespace)
	if err != nil {
		return false, err
	}
	if existing > 0 {
		return false, nil
	}

	if _, err := r.writeValues(ctx, namespace, values, false); err != nil {
		return false, err
	}
	return true, nil
}

func (r *SQLiteRepository) Read(ctx context.Context, pluginID string, keys []string) (map[string]any, error) {
	if len(keys) == 0 {
		return map[string]any{}, nil
	}

	namespace := namespaceForPlugin(pluginID)
	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)+1)
	args = append(args, namespace)
	seen := make(map[string]struct{}, len(keys))
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
		placeholders = append(placeholders, "?")
		args = append(args, key)
	}
	if len(normalized) == 0 {
		return map[string]any{}, nil
	}

	query := fmt.Sprintf(
		`SELECT key, value_json FROM system_configs WHERE namespace = ? AND key IN (%s) ORDER BY key ASC`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query system configs for %s: %w", pluginID, err)
	}
	defer rows.Close()

	values := make(map[string]any)
	for rows.Next() {
		var key string
		var raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, fmt.Errorf("scan system config row: %w", err)
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("decode system config %s: %w", key, err)
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system config rows: %w", err)
	}
	return values, nil
}

func (r *SQLiteRepository) Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error) {
	namespace := namespaceForPlugin(pluginID)
	return r.writeValues(ctx, namespace, values, true)
}

func (r *SQLiteRepository) namespaceCount(ctx context.Context, namespace string) (int, error) {
	var count int
	if err := r.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_configs WHERE namespace = ?`, namespace).Scan(&count); err != nil {
		return 0, fmt.Errorf("count system configs for %s: %w", namespace, err)
	}
	return count, nil
}

func (r *SQLiteRepository) writeValues(ctx context.Context, namespace string, values map[string]any, overwrite bool) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return []string{}, nil
	}

	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin system config tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	statement := `INSERT INTO system_configs (namespace, key, value_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(namespace, key) DO UPDATE SET
			value_json = excluded.value_json,
			updated_at = excluded.updated_at`
	if !overwrite {
		statement = `INSERT INTO system_configs (namespace, key, value_json, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(namespace, key) DO NOTHING`
	}

	for _, key := range keys {
		raw, err := json.Marshal(values[key])
		if err != nil {
			return nil, fmt.Errorf("marshal system config %s: %w", key, err)
		}
		if _, err := tx.ExecContext(ctx, statement, namespace, key, string(raw), now); err != nil {
			return nil, fmt.Errorf("upsert system config %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit system config tx: %w", err)
	}
	return keys, nil
}

func namespaceForPlugin(pluginID string) string {
	return fmt.Sprintf("plugin:%s:settings", strings.TrimSpace(pluginID))
}
