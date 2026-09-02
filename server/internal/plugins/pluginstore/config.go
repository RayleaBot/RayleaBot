package pluginstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type ConfigRepository interface {
	SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error)
	Read(ctx context.Context, pluginID string, keys []string) (map[string]any, error)
	ReadAll(ctx context.Context, pluginID string) (map[string]any, error)
	Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error)
}

type ConfigSQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	read   *sql.DB
	write  *sql.DB
}

func NewConfigSQLiteRepository(store *storage.Store) (*ConfigSQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &ConfigSQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		read:   store.Read,
		write:  store.Write,
	}, nil
}

func (r *ConfigSQLiteRepository) SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}
	written, err := r.writeValues(ctx, namespaceForPlugin(pluginID), values, false)
	if err != nil {
		return false, err
	}
	return len(written) > 0, nil
}

func (r *ConfigSQLiteRepository) Read(ctx context.Context, pluginID string, keys []string) (map[string]any, error) {
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

	return scanConfigRows(rows)
}

func (r *ConfigSQLiteRepository) ReadAll(ctx context.Context, pluginID string) (map[string]any, error) {
	namespace := namespaceForPlugin(pluginID)
	rows, err := r.read.QueryContext(
		ctx,
		`SELECT key, value_json FROM system_configs WHERE namespace = ? ORDER BY key ASC`,
		namespace,
	)
	if err != nil {
		return nil, fmt.Errorf("query all system configs for %s: %w", pluginID, err)
	}
	defer rows.Close()

	return scanConfigRows(rows)
}

func (r *ConfigSQLiteRepository) Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error) {
	namespace := namespaceForPlugin(pluginID)
	return r.writeValues(ctx, namespace, values, true)
}

func (r *ConfigSQLiteRepository) writeValues(ctx context.Context, namespace string, values map[string]any, overwrite bool) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	keys := sortedConfigKeys(values)
	if len(keys) == 0 {
		return []string{}, nil
	}

	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin system config tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := r.writeQ.WithTx(tx)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	written := make([]string, 0, len(keys))
	for _, key := range keys {
		raw, err := json.Marshal(values[key])
		if err != nil {
			return nil, fmt.Errorf("marshal system config %s: %w", key, err)
		}
		if overwrite {
			if err := q.UpsertConfig(ctx, sqlcgen.UpsertConfigParams{
				Namespace: namespace,
				Key:       key,
				ValueJson: string(raw),
				UpdatedAt: now,
			}); err != nil {
				return nil, fmt.Errorf("upsert system config %s: %w", key, err)
			}
			written = append(written, key)
		} else {
			result, err := tx.ExecContext(ctx, `
INSERT INTO system_configs (namespace, key, value_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace, key) DO NOTHING`, namespace, key, string(raw), now)
			if err != nil {
				return nil, fmt.Errorf("seed system config %s: %w", key, err)
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return nil, fmt.Errorf("read seeded system config result for %s: %w", key, err)
			}
			if rowsAffected > 0 {
				written = append(written, key)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit system config tx: %w", err)
	}
	return written, nil
}

func MergeValues(defaults, persisted map[string]any) map[string]any {
	merged := cloneValues(defaults)
	for key, value := range persisted {
		merged[key] = cloneValue(value)
	}
	return merged
}

func cloneValues(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneValues(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func scanConfigRows(rows *sql.Rows) (map[string]any, error) {
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

func sortedConfigKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func namespaceForPlugin(pluginID string) string {
	return fmt.Sprintf("plugin:%s:settings", strings.TrimSpace(pluginID))
}
