package tasks

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

// Repository defines the persistence interface for task snapshots.
type Repository interface {
	SaveTask(ctx context.Context, snapshot Snapshot) error
	LoadTasks(ctx context.Context) ([]Snapshot, error)
	DeleteTask(ctx context.Context, taskID string) error
}

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

// NewSQLiteRepository creates a new SQLite-backed task repository.
func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
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

	if err := r.writeQ.SaveTask(ctx, sqlcgen.SaveTaskParams{
		TaskID:     snapshot.TaskID,
		TaskType:   snapshot.TaskType,
		Status:     string(snapshot.Status),
		Progress:   int64(snapshot.Progress),
		Summary:    snapshot.Summary,
		StartedAt:  toNullString(formatOptionalTime(snapshot.StartedAt)),
		FinishedAt: toNullString(formatOptionalTime(snapshot.FinishedAt)),
		ResultJson: toNullString(resultJSON),
		ErrorJson:  toNullString(errorJSON),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("upsert task %s: %w", snapshot.TaskID, err)
	}
	return nil
}

// LoadTasks loads all task snapshots from the database, ordered by creation time.
func (r *SQLiteRepository) LoadTasks(ctx context.Context) ([]Snapshot, error) {
	rows, err := r.readQ.LoadTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	snapshots := make([]Snapshot, 0, len(rows))
	for _, row := range rows {
		s := Snapshot{
			TaskID:     row.TaskID,
			TaskType:   row.TaskType,
			Status:     Status(row.Status),
			Progress:   int(row.Progress),
			Summary:    row.Summary,
			StartedAt:  parseOptionalTime(row.StartedAt),
			FinishedAt: parseOptionalTime(row.FinishedAt),
		}

		if row.ResultJson.Valid && row.ResultJson.String != "" {
			var result ResultSummary
			if err := json.Unmarshal([]byte(row.ResultJson.String), &result); err == nil {
				s.Result = &result
			}
		}
		if row.ErrorJson.Valid && row.ErrorJson.String != "" {
			var errSummary ErrorSummary
			if err := json.Unmarshal([]byte(row.ErrorJson.String), &errSummary); err == nil {
				s.Error = &errSummary
			}
		}

		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// DeleteTask removes a task snapshot from the database.
func (r *SQLiteRepository) DeleteTask(ctx context.Context, taskID string) error {
	if err := r.writeQ.DeleteTask(ctx, taskID); err != nil {
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

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
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
