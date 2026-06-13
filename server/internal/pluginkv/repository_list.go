package pluginkv

import (
	"context"
	"fmt"
	"strings"
)

// List uses ESCAPE clause not supported by sqlc's SQLite parser; kept as hand-written SQL.
func (r *SQLiteRepository) List(ctx context.Context, pluginID, prefix string) ([]string, error) {
	rows, err := r.read.QueryContext(
		ctx,
		`SELECT key
		 FROM plugin_kv
		 WHERE plugin_id = ? AND key LIKE ? ESCAPE '\'
		 ORDER BY key ASC`,
		strings.TrimSpace(pluginID),
		escapeLike(prefix)+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("list plugin kv keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan plugin kv key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin kv keys: %w", err)
	}
	return keys, nil
}
