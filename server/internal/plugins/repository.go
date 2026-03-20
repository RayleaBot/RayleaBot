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
