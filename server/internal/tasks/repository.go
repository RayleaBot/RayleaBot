package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rayleabot/server/internal/storage"
)

// Repository defines the persistence interface for task snapshots.
type Repository interface {
	SaveTask(ctx context.Context, snapshot Snapshot) error
	LoadTasks(ctx context.Context) ([]Snapshot, error)
	DeleteTask(ctx context.Context, taskID string) error
}

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	read  *sql.DB
	write *sql.DB
}

// NewSQLiteRepository creates a new SQLite-backed task repository.
func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}

// SaveTask upserts a task snapshot into the database.
func (r *SQLiteRepository) SaveTask(ctx context.Context, snapshot Snapshot) error {
	resultJSON, err := marshalOptionalJSON(snapshot.Result)
	if err != nil {
		return fmt.Errorf("marshal task result for %s: %w", snapshot.TaskID, err)
	}
	errorJSON, err := marshalOptionalJSON(snapshot.Error)
	if err != nil {
		return fmt.Errorf("marshal task error for %s: %w", snapshot.TaskID, err)
	}

	if _, err := r.write.ExecContext(ctx,
		`INSERT INTO tasks (task_id, task_type, status, progress, summary, started_at, finished_at, result_json, error_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			status = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.status
				ELSE excluded.status
			END,
			progress = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.progress
				ELSE excluded.progress
			END,
			summary = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.summary
				ELSE excluded.summary
			END,
			started_at = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.started_at
				ELSE excluded.started_at
			END,
			finished_at = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.finished_at
				ELSE excluded.finished_at
			END,
			result_json = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.result_json
				ELSE excluded.result_json
			END,
			error_json = CASE
				WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
					AND excluded.status IN ('pending', 'running')
				THEN tasks.error_json
				ELSE excluded.error_json
			END`,
		snapshot.TaskID,
		snapshot.TaskType,
		string(snapshot.Status),
		snapshot.Progress,
		snapshot.Summary,
		formatOptionalTime(snapshot.StartedAt),
		formatOptionalTime(snapshot.FinishedAt),
		resultJSON,
		errorJSON,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert task %s: %w", snapshot.TaskID, err)
	}
	return nil
}

// LoadTasks loads all task snapshots from the database, ordered by creation time.
func (r *SQLiteRepository) LoadTasks(ctx context.Context) ([]Snapshot, error) {
	rows, err := r.read.QueryContext(ctx,
		`SELECT task_id, task_type, status, progress, summary, started_at, finished_at, result_json, error_json
		FROM tasks ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var s Snapshot
		var status string
		var startedAt, finishedAt sql.NullString
		var resultJSON, errorJSON sql.NullString

		if err := rows.Scan(&s.TaskID, &s.TaskType, &status, &s.Progress, &s.Summary,
			&startedAt, &finishedAt, &resultJSON, &errorJSON); err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}

		s.Status = Status(status)
		s.StartedAt = parseOptionalTime(startedAt)
		s.FinishedAt = parseOptionalTime(finishedAt)

		if resultJSON.Valid && resultJSON.String != "" {
			var result ResultSummary
			if err := json.Unmarshal([]byte(resultJSON.String), &result); err == nil {
				s.Result = &result
			}
		}
		if errorJSON.Valid && errorJSON.String != "" {
			var errSummary ErrorSummary
			if err := json.Unmarshal([]byte(errorJSON.String), &errSummary); err == nil {
				s.Error = &errSummary
			}
		}

		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

// DeleteTask removes a task snapshot from the database.
func (r *SQLiteRepository) DeleteTask(ctx context.Context, taskID string) error {
	if _, err := r.write.ExecContext(ctx, `DELETE FROM tasks WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task %s: %w", taskID, err)
	}
	return nil
}

func marshalOptionalJSON(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseOptionalTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
