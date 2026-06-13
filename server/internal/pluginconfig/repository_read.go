package pluginconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

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

	return scanConfigRows(rows)
}

func (r *SQLiteRepository) ReadAll(ctx context.Context, pluginID string) (map[string]any, error) {
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
