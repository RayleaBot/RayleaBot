package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

// Repository defines the persistence interface for scheduled jobs.
type Repository interface {
	SaveJob(ctx context.Context, job Job) error
	LoadJobs(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, jobID string) error
	DeleteJobsByPlugin(ctx context.Context, pluginID string) error
	RecordJobRunResult(ctx context.Context, result RunResult) error
	UpdateJobSchedule(ctx context.Context, job Job) error
}

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

// NewSQLiteRepository creates a new SQLite-backed scheduler repository.
func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
	}, nil
}

// SaveJob upserts a scheduled job.
func (r *SQLiteRepository) SaveJob(ctx context.Context, job Job) error {
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

	if err := r.writeQ.SaveJob(ctx, sqlcgen.SaveJobParams{
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
	}); err != nil {
		return fmt.Errorf("upsert scheduler job %s: %w", job.JobID, err)
	}
	return nil
}

// LoadJobs loads all scheduled jobs from the database.
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

func (r *SQLiteRepository) RecordJobRunResult(ctx context.Context, result RunResult) error {
	if result.OccurredAt.IsZero() {
		result.OccurredAt = time.Now().UTC()
	}
	if result.Duration < 0 {
		result.Duration = 0
	}
	outcome := normalizeRunOutcome(result.Outcome)
	lastRun := result.OccurredAt.UTC().Format(time.RFC3339Nano)

	if outcome == RunOutcomeSuccess {
		if err := r.writeQ.RecordJobRunSuccess(ctx, sqlcgen.RecordJobRunSuccessParams{
			LastRun:        sql.NullString{String: lastRun, Valid: true},
			LastDurationMs: result.Duration.Milliseconds(),
			UpdatedAt:      lastRun,
			JobID:          result.JobID,
		}); err != nil {
			return fmt.Errorf("record scheduler job run result %s: %w", result.JobID, err)
		}
		return nil
	}

	var failed, timeoutCount, retry, other int64
	switch outcome {
	case RunOutcomeFailed:
		failed = 1
	case RunOutcomeTimeout:
		timeoutCount = 1
	case RunOutcomeRetry:
		retry = 1
	case RunOutcomeOther:
		other = 1
	}

	if err := r.writeQ.RecordJobRunFailure(ctx, sqlcgen.RecordJobRunFailureParams{
		LastRun:          sql.NullString{String: lastRun, Valid: true},
		LastDurationMs:   result.Duration.Milliseconds(),
		LastErrorCode:    DisplayLabel(result.ErrorCode, string(outcome)),
		LastErrorMessage: DisplayLabel(result.ErrorText, string(outcome)),
		LastErrorAt:      sql.NullString{String: lastRun, Valid: true},
		FailureCount:     failed,
		TimeoutCount:     timeoutCount,
		RetryCount:       retry,
		OtherCount:       other,
		UpdatedAt:        lastRun,
		JobID:            result.JobID,
	}); err != nil {
		return fmt.Errorf("record scheduler job run result %s: %w", result.JobID, err)
	}
	return nil
}

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

// DeleteJob removes a scheduled job by ID.
func (r *SQLiteRepository) DeleteJob(ctx context.Context, jobID string) error {
	if err := r.writeQ.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete scheduler job %s: %w", jobID, err)
	}
	return nil
}

// DeleteJobsByPlugin removes all scheduled jobs for a given plugin.
func (r *SQLiteRepository) DeleteJobsByPlugin(ctx context.Context, pluginID string) error {
	if err := r.writeQ.DeleteJobsByPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("delete scheduler jobs for plugin %s: %w", pluginID, err)
	}
	return nil
}
