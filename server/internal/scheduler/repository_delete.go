package scheduler

import (
	"context"
	"fmt"
)

func (r *SQLiteRepository) DeleteJob(ctx context.Context, jobID string) error {
	if err := r.writeQ.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete scheduler job %s: %w", jobID, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteJobsByPlugin(ctx context.Context, pluginID string) error {
	if err := r.writeQ.DeleteJobsByPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("delete scheduler jobs for plugin %s: %w", pluginID, err)
	}
	return nil
}
