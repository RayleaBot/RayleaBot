package logging

import (
	"context"
	"fmt"
	"strings"
)

func (r *SQLiteRepository) ListPage(ctx context.Context, query PageQuery) (PageResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	direction := query.Direction
	if direction == "" {
		direction = PageDirectionOlder
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
		return PageResult{}, err
	}

	cursor, err := decodeLogCursor(query.Cursor)
	if err != nil {
		return PageResult{}, err
	}
	if cursor == nil {
		direction = PageDirectionOlder
	}
	if cursor != nil {
		switch direction {
		case PageDirectionOlder:
			clauses = append(clauses, "("+logTimestampExpr+" < julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id < ?))")
			args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.RowID)
		case PageDirectionNewer:
			clauses = append(clauses, "("+logTimestampExpr+" > julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id > ?))")
			args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.RowID)
		default:
			return PageResult{}, fmt.Errorf("%w: unsupported direction %q", ErrInvalidCursor, direction)
		}
	}

	orderClause := "ORDER BY " + logTimestampExpr + " DESC, id DESC"
	if direction == PageDirectionNewer {
		orderClause = "ORDER BY " + logTimestampExpr + " ASC, id ASC"
	}
	args = append(args, limit+1)

	rows, err := r.read.QueryContext(
		ctx,
		`SELECT id, log_id, boot_id, ts, level, source, message, plugin_id, request_id
		 FROM management_logs
		 WHERE `+strings.Join(clauses, " AND ")+`
		 `+orderClause+`
		 LIMIT ?`,
		args...,
	)
	if err != nil {
		return PageResult{}, fmt.Errorf("query management log page: %w", err)
	}
	defer rows.Close()

	entries := make([]pagedSummary, 0, limit+1)
	for rows.Next() {
		entry, err := scanPagedSummary(rows)
		if err != nil {
			return PageResult{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return PageResult{}, fmt.Errorf("iterate management log page: %w", err)
	}

	if direction == PageDirectionNewer {
		for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	result := PageResult{
		Items: make([]Summary, 0, len(entries)),
		Page: PageInfo{
			Limit: limit,
		},
	}
	if len(entries) == 0 {
		return result, nil
	}

	for _, entry := range entries {
		result.Items = append(result.Items, entry.Summary)
	}

	oldest := entries[len(entries)-1]
	newest := entries[0]

	hasOlder, err := r.hasRows(ctx, query.filterSpec(), logBoundaryOlder, oldest.marker())
	if err != nil {
		return PageResult{}, err
	}
	hasNewer, err := r.hasRows(ctx, query.filterSpec(), logBoundaryNewer, newest.marker())
	if err != nil {
		return PageResult{}, err
	}

	result.Page.HasOlder = hasOlder
	result.Page.HasNewer = hasNewer
	if hasOlder {
		cursor := encodeLogCursor(oldest.marker())
		result.Page.OlderCursor = &cursor
	}
	if hasNewer {
		cursor := encodeLogCursor(newest.marker())
		result.Page.NewerCursor = &cursor
	}

	return result, nil
}

func (q PageQuery) filterSpec() filterSpec {
	return filterSpec{
		Level:     q.Level,
		Levels:    q.Levels,
		Source:    q.Source,
		Protocol:  q.Protocol,
		PluginID:  q.PluginID,
		PluginIDs: q.PluginIDs,
		RequestID: q.RequestID,
		BootID:    q.BootID,
		StartAt:   q.StartAt,
		EndAt:     q.EndAt,
	}
}
