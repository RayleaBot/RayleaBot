package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) UpdateJobSchedule(ctx context.Context, job Job) error {
	updatedAt := job.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	if err := r.writeQ.UpdateJobSchedule(ctx, sqlcgen.UpdateJobScheduleParams{
		NextRun:   job.NextRun.UTC().Format(time.RFC3339Nano),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		JobID:     job.JobID,
	}); err != nil {
		return fmt.Errorf("update scheduler job schedule %s: %w", job.JobID, err)
	}
	return nil
}
