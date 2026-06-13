package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) SaveJob(ctx context.Context, job Job) error {
	if err := r.writeQ.SaveJob(ctx, saveJobParams(job)); err != nil {
		return fmt.Errorf("upsert scheduler job %s: %w", job.JobID, err)
	}
	return nil
}

func saveJobParams(job Job) sqlcgen.SaveJobParams {
	payload := "{}"
	if job.Payload != nil {
		payload = string(job.Payload)
	}

	enabled := int64(0)
	if job.Enabled {
		enabled = 1
	}

	var lastRun sql.NullString
	if job.LastRun != nil {
		lastRun = sql.NullString{String: job.LastRun.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastErrorCode, lastErrorMessage string
	var lastErrorAt sql.NullString
	if job.LastError != nil {
		lastErrorCode = DisplayLabel(job.LastError.Code, "error")
		lastErrorMessage = DisplayLabel(job.LastError.Message, lastErrorCode)
		if !job.LastError.At.IsZero() {
			lastErrorAt = sql.NullString{String: job.LastError.At.UTC().Format(time.RFC3339Nano), Valid: true}
		}
	}

	return sqlcgen.SaveJobParams{
		JobID:            job.JobID,
		PluginID:         job.PluginID,
		LogLabel:         job.LogLabel,
		CronExpr:         job.CronExpr,
		Payload:          payload,
		Enabled:          enabled,
		NextRun:          job.NextRun.UTC().Format(time.RFC3339Nano),
		LastRun:          lastRun,
		CreatedAt:        job.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:        job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		LastDurationMs:   job.LastDurationMS,
		LastErrorCode:    lastErrorCode,
		LastErrorMessage: lastErrorMessage,
		LastErrorAt:      lastErrorAt,
		SuccessCount:     job.RunStats.Success,
		FailureCount:     job.RunStats.Failed,
		TimeoutCount:     job.RunStats.Timeout,
		RetryCount:       job.RunStats.Retry,
		OtherCount:       job.RunStats.Other,
	}
}
