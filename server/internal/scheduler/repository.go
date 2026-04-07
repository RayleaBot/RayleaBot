package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rayleabot/server/internal/sqlcgen"
	"rayleabot/server/internal/storage"
)

// Repository defines the persistence interface for scheduled jobs.
type Repository interface {
	SaveJob(ctx context.Context, job Job) error
	LoadJobs(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, jobID string) error
	DeleteJobsByPlugin(ctx context.Context, pluginID string) error
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

	if err := r.writeQ.SaveJob(ctx, sqlcgen.SaveJobParams{
		JobID:     job.JobID,
		PluginID:  job.PluginID,
		CronExpr:  job.CronExpr,
		Payload:   payload,
		Enabled:   enabled,
		NextRun:   job.NextRun.UTC().Format(time.RFC3339Nano),
		LastRun:   lastRun,
		CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339Nano),
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
			JobID:    row.JobID,
			PluginID: row.PluginID,
			CronExpr: row.CronExpr,
			Payload:  json.RawMessage(row.Payload),
			Enabled:  row.Enabled != 0,
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
