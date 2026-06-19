package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (r *SQLiteRepository) ListSummaries(ctx context.Context, query logging.Query) ([]logging.Summary, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	clauses, args, err := buildLogFilterClauses(filterSpec{
		Level:     query.Level,
		Levels:    query.Levels,
		Source:    query.Source,
		Protocol:  query.Protocol,
		PluginID:  query.PluginID,
		PluginIDs: query.PluginIDs,
		RequestID: query.RequestID,
		BootID:    query.BootID,
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
	})
	if err != nil {
		return nil, err
	}
	args = append(args, limit)

	rows, err := r.read.QueryContext(
		ctx,
		`SELECT id, log_id, boot_id, ts, level, source, message, plugin_id, request_id
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

	items := make([]logging.Summary, 0, limit)
	for rows.Next() {
		var rowID int64
		var summary logging.Summary
		if err := rows.Scan(
			&rowID,
			&summary.LogID,
			&summary.BootID,
			&summary.Timestamp,
			&summary.Level,
			&summary.Source,
			&summary.Message,
			&summary.PluginID,
			&summary.RequestID,
		); err != nil {
			return nil, fmt.Errorf("scan management log summary: %w", err)
		}
		summary = logging.NormalizeSummary(summary)
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
