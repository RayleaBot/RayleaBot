package logging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type pagedSummary struct {
	RowID   int64
	Summary Summary
}

func (p pagedSummary) marker() logCursor {
	return logCursor{
		RowID:     p.RowID,
		Timestamp: p.Summary.Timestamp,
	}
}

type logBoundary string

const (
	logBoundaryOlder logBoundary = "older"
	logBoundaryNewer logBoundary = "newer"
)

func scanPagedSummary(scanner interface{ Scan(...any) error }) (pagedSummary, error) {
	var entry pagedSummary
	if err := scanner.Scan(
		&entry.RowID,
		&entry.Summary.LogID,
		&entry.Summary.BootID,
		&entry.Summary.Timestamp,
		&entry.Summary.Level,
		&entry.Summary.Source,
		&entry.Summary.Message,
		&entry.Summary.PluginID,
		&entry.Summary.RequestID,
	); err != nil {
		return pagedSummary{}, fmt.Errorf("scan management log summary: %w", err)
	}
	entry.Summary = NormalizeSummary(entry.Summary)
	return entry, nil
}

func (r *SQLiteRepository) hasRows(ctx context.Context, spec filterSpec, boundary logBoundary, marker logCursor) (bool, error) {
	clauses, args, err := buildLogFilterClauses(spec)
	if err != nil {
		return false, err
	}
	switch boundary {
	case logBoundaryOlder:
		clauses = append(clauses, "("+logTimestampExpr+" < julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id < ?))")
	case logBoundaryNewer:
		clauses = append(clauses, "("+logTimestampExpr+" > julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id > ?))")
	default:
		return false, fmt.Errorf("unsupported log boundary %q", boundary)
	}
	args = append(args, marker.Timestamp, marker.Timestamp, marker.RowID)

	var exists int
	if err := r.read.QueryRowContext(
		ctx,
		`SELECT 1
		 FROM management_logs
		 WHERE `+strings.Join(clauses, " AND ")+`
		 LIMIT 1`,
		args...,
	).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query management log boundary: %w", err)
	}
	return true, nil
}
