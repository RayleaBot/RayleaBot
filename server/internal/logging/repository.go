package logging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Query struct {
	Level     string
	Source    string
	Protocol  string
	PluginID  string
	RequestID string
	Limit     int
}

type Repository interface {
	SaveSummary(context.Context, Summary) error
	ListSummaries(context.Context, Query) ([]Summary, error)
	PruneOlderThan(context.Context, time.Time) error
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	read   *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		read:   store.Read,
	}, nil
}

func (r *SQLiteRepository) SaveSummary(ctx context.Context, summary Summary) error {
	summary = NormalizeSummary(summary)

	if err := r.writeQ.InsertLogSummary(ctx, sqlcgen.InsertLogSummaryParams{
		Ts:        summary.Timestamp,
		Level:     strings.ToLower(strings.TrimSpace(summary.Level)),
		Source:    strings.TrimSpace(summary.Source),
		Message:   strings.TrimSpace(summary.Message),
		PluginID:  strings.TrimSpace(summary.PluginID),
		RequestID: strings.TrimSpace(summary.RequestID),
	}); err != nil {
		return fmt.Errorf("insert management log summary: %w", err)
	}
	return nil
}

// ListSummaries uses dynamic WHERE clauses not supported by sqlc; kept as hand-written SQL.
func (r *SQLiteRepository) ListSummaries(ctx context.Context, query Query) ([]Summary, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	clauses := []string{"1 = 1"}
	args := make([]any, 0, 5)
	if query.Level != "" {
		clauses = append(clauses, "level = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(query.Level)))
	}
	if query.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, strings.TrimSpace(query.Source))
	}
	if query.Protocol != "" {
		sources := protocolSources(query.Protocol)
		if len(sources) == 0 {
			return []Summary{}, nil
		}
		clauses = append(clauses, "source IN (?, ?, ?)")
		for _, source := range sources {
			args = append(args, source)
		}
	}
	if query.PluginID != "" {
		clauses = append(clauses, "plugin_id = ?")
		args = append(args, strings.TrimSpace(query.PluginID))
	}
	if query.RequestID != "" {
		clauses = append(clauses, "request_id = ?")
		args = append(args, strings.TrimSpace(query.RequestID))
	}
	args = append(args, limit)

	rows, err := r.read.QueryContext(
		ctx,
		`SELECT ts, level, source, message, plugin_id, request_id
		 FROM management_logs
		 WHERE `+strings.Join(clauses, " AND ")+`
		 ORDER BY ts DESC, id DESC
		 LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query management log summaries: %w", err)
	}
	defer rows.Close()

	items := make([]Summary, 0, limit)
	for rows.Next() {
		var summary Summary
		if err := rows.Scan(
			&summary.Timestamp,
			&summary.Level,
			&summary.Source,
			&summary.Message,
			&summary.PluginID,
			&summary.RequestID,
		); err != nil {
			return nil, fmt.Errorf("scan management log summary: %w", err)
		}
		summary = NormalizeSummary(summary)
		items = append(items, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate management log summaries: %w", err)
	}

	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, nil
}

func (r *SQLiteRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}

	if err := r.writeQ.PruneLogsBefore(ctx, cutoff.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("prune management log summaries: %w", err)
	}
	return nil
}
