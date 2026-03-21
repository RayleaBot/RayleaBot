package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	read  *sql.DB
	write *sql.DB
}

// NewSQLiteRepository creates a new SQLite-backed scheduler repository.
func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}

// SaveJob upserts a scheduled job.
func (r *SQLiteRepository) SaveJob(ctx context.Context, job Job) error {
	payload := "{}"
	if job.Payload != nil {
		payload = string(job.Payload)
	}

	enabled := 0
	if job.Enabled {
		enabled = 1
	}

	lastRun := ""
	if job.LastRun != nil {
		lastRun = job.LastRun.UTC().Format(time.RFC3339Nano)
	}

	if _, err := r.write.ExecContext(ctx,
		`INSERT INTO scheduler_jobs (job_id, plugin_id, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			cron_expr = excluded.cron_expr,
			payload = excluded.payload,
			enabled = excluded.enabled,
			next_run = excluded.next_run,
			last_run = excluded.last_run,
			updated_at = excluded.updated_at`,
		job.JobID,
		job.PluginID,
		job.CronExpr,
		payload,
		enabled,
		job.NextRun.UTC().Format(time.RFC3339Nano),
		lastRun,
		job.CreatedAt.UTC().Format(time.RFC3339Nano),
		job.UpdatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert scheduler job %s: %w", job.JobID, err)
	}
	return nil
}

// LoadJobs loads all scheduled jobs from the database.
func (r *SQLiteRepository) LoadJobs(ctx context.Context) ([]Job, error) {
	rows, err := r.read.QueryContext(ctx,
		`SELECT job_id, plugin_id, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at
		FROM scheduler_jobs ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query scheduler jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var payload string
		var enabled int
		var nextRun, createdAt, updatedAt string
		var lastRun sql.NullString

		if err := rows.Scan(&j.JobID, &j.PluginID, &j.CronExpr, &payload,
			&enabled, &nextRun, &lastRun, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan scheduler job row: %w", err)
		}

		j.Payload = json.RawMessage(payload)
		j.Enabled = enabled != 0

		if t, err := time.Parse(time.RFC3339Nano, nextRun); err == nil {
			j.NextRun = t.UTC()
		}
		if lastRun.Valid && lastRun.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, lastRun.String); err == nil {
				utc := t.UTC()
				j.LastRun = &utc
			}
		}
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			j.CreatedAt = t.UTC()
		}
		if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
			j.UpdatedAt = t.UTC()
		}

		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// DeleteJob removes a scheduled job by ID.
func (r *SQLiteRepository) DeleteJob(ctx context.Context, jobID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM scheduler_jobs WHERE job_id = ?`, jobID); err != nil {
		return fmt.Errorf("delete scheduler job %s: %w", jobID, err)
	}
	return nil
}

// DeleteJobsByPlugin removes all scheduled jobs for a given plugin.
func (r *SQLiteRepository) DeleteJobsByPlugin(ctx context.Context, pluginID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM scheduler_jobs WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("delete scheduler jobs for plugin %s: %w", pluginID, err)
	}
	return nil
}
