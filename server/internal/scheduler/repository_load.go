package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (r *SQLiteRepository) LoadJobs(ctx context.Context) ([]Job, error) {
	rows, err := r.readQ.LoadJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("query scheduler jobs: %w", err)
	}

	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		j := Job{
			JobID:          row.JobID,
			PluginID:       row.PluginID,
			LogLabel:       row.LogLabel,
			CronExpr:       row.CronExpr,
			Payload:        json.RawMessage(row.Payload),
			Enabled:        row.Enabled != 0,
			LastDurationMS: row.LastDurationMs,
			RunStats: RunStats{
				Success: row.SuccessCount,
				Failed:  row.FailureCount,
				Timeout: row.TimeoutCount,
				Retry:   row.RetryCount,
				Other:   row.OtherCount,
			},
		}

		if t, err := time.Parse(time.RFC3339Nano, row.NextRun); err == nil {
			j.NextRun = t.UTC()
		}
		if row.LastRun.Valid && row.LastRun.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, row.LastRun.String); err == nil {
				utc := t.UTC()
				j.LastRun = &utc
			}
		}
		if row.LastErrorCode != "" || row.LastErrorMessage != "" {
			j.LastError = &RunError{
				Code:    row.LastErrorCode,
				Message: row.LastErrorMessage,
			}
			if row.LastErrorAt.Valid && row.LastErrorAt.String != "" {
				if t, err := time.Parse(time.RFC3339Nano, row.LastErrorAt.String); err == nil {
					j.LastError.At = t.UTC()
				}
			}
		}
		if t, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err == nil {
			j.CreatedAt = t.UTC()
		}
		if t, err := time.Parse(time.RFC3339Nano, row.UpdatedAt); err == nil {
			j.UpdatedAt = t.UTC()
		}

		jobs = append(jobs, j)
	}
	return jobs, nil
}
