package pluginrepository

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

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
