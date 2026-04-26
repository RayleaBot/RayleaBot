package logging

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Query struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
	Limit     int
}

type PageDirection string

const (
	PageDirectionOlder PageDirection = "older"
	PageDirectionNewer PageDirection = "newer"
)

type PageQuery struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
	Limit     int
	Cursor    string
	Direction PageDirection
}

type PageInfo struct {
	Limit       int     `json:"limit"`
	HasOlder    bool    `json:"has_older"`
	HasNewer    bool    `json:"has_newer"`
	OlderCursor *string `json:"older_cursor"`
	NewerCursor *string `json:"newer_cursor"`
}

type PageResult struct {
	Items []Summary `json:"items"`
	Page  PageInfo  `json:"page"`
}

var ErrLogNotFound = errors.New("management log not found")
var ErrInvalidCursor = errors.New("management log cursor is invalid")

type Repository interface {
	SaveSummary(context.Context, Summary) error
	ListSummaries(context.Context, Query) ([]Summary, error)
	ListPage(context.Context, PageQuery) (PageResult, error)
	GetSummary(context.Context, string) (Summary, error)
	PruneOlderThan(context.Context, time.Time) error
}

type SQLiteRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}

func (r *SQLiteRepository) SaveSummary(ctx context.Context, summary Summary) error {
	summary = NormalizeSummary(summary)
	detailsJSON, err := encodeDetailsJSON(summary.Details)
	if err != nil {
		return fmt.Errorf("encode management log details: %w", err)
	}

	if _, err := r.write.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO management_logs (log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		summary.LogID,
		summary.BootID,
		summary.Timestamp,
		strings.ToLower(strings.TrimSpace(summary.Level)),
		strings.TrimSpace(summary.Source),
		strings.TrimSpace(summary.Message),
		strings.TrimSpace(summary.PluginID),
		strings.TrimSpace(summary.RequestID),
		detailsJSON,
	); err != nil {
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

	items := make([]Summary, 0, limit)
	for rows.Next() {
		var rowID int64
		var summary Summary
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

	hasOlder, err := r.hasRows(ctx, filterSpec{
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
	}, logBoundaryOlder, oldest.marker())
	if err != nil {
		return PageResult{}, err
	}
	hasNewer, err := r.hasRows(ctx, filterSpec{
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
	}, logBoundaryNewer, newest.marker())
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

func (r *SQLiteRepository) GetSummary(ctx context.Context, logID string) (Summary, error) {
	row := r.read.QueryRowContext(
		ctx,
		`SELECT log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json
		 FROM management_logs
		 WHERE log_id = ?
		 LIMIT 1`,
		strings.TrimSpace(logID),
	)
	var item struct {
		LogID      string
		BootID     string
		Timestamp  string
		Level      string
		Source     string
		Message    string
		PluginID   string
		RequestID  string
		DetailsRaw string
	}
	if err := row.Scan(
		&item.LogID,
		&item.BootID,
		&item.Timestamp,
		&item.Level,
		&item.Source,
		&item.Message,
		&item.PluginID,
		&item.RequestID,
		&item.DetailsRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Summary{}, ErrLogNotFound
		}
		return Summary{}, fmt.Errorf("query management log detail: %w", err)
	}

	details, err := decodeDetailsJSON(item.DetailsRaw)
	if err != nil {
		return Summary{}, fmt.Errorf("decode management log detail %s: %w", item.LogID, err)
	}

	return NormalizeSummary(Summary{
		BootID:    item.BootID,
		LogID:     item.LogID,
		Timestamp: item.Timestamp,
		Level:     item.Level,
		Source:    item.Source,
		Message:   item.Message,
		PluginID:  item.PluginID,
		RequestID: item.RequestID,
		Details:   details,
	}), nil
}

func (r *SQLiteRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}

	if _, err := r.write.ExecContext(ctx, `DELETE FROM management_logs WHERE `+logTimestampExpr+` < julianday(?)`, cutoff.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("prune management log summaries: %w", err)
	}
	return nil
}

type filterSpec struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
}

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

type logCursor struct {
	Version   int    `json:"v"`
	RowID     int64  `json:"row_id"`
	Timestamp string `json:"ts"`
}

type logBoundary string

const (
	logBoundaryOlder logBoundary = "older"
	logBoundaryNewer logBoundary = "newer"
)

const logTimestampExpr = "julianday(ts)"

func buildLogFilterClauses(spec filterSpec) ([]string, []any, error) {
	clauses := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if levels := normalizeFilterValues(spec.Level, spec.Levels, true); len(levels) > 0 {
		clauses, args = appendStringSetClause(clauses, args, "level", levels)
	}
	if spec.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, strings.TrimSpace(spec.Source))
	}
	if spec.Protocol != "" {
		sources := protocolSources(spec.Protocol)
		if len(sources) == 0 {
			return []string{"1 = 0"}, args, nil
		}
		placeholders := make([]string, 0, len(sources))
		for _, source := range sources {
			placeholders = append(placeholders, "?")
			args = append(args, source)
		}
		clauses = append(clauses, "source IN ("+strings.Join(placeholders, ", ")+")")
	}
	if pluginIDs := normalizeFilterValues(spec.PluginID, spec.PluginIDs, false); len(pluginIDs) > 0 {
		clauses, args = appendStringSetClause(clauses, args, "plugin_id", pluginIDs)
	}
	if spec.RequestID != "" {
		clauses = append(clauses, "request_id = ?")
		args = append(args, strings.TrimSpace(spec.RequestID))
	}
	if spec.BootID != "" {
		clauses = append(clauses, "boot_id = ?")
		args = append(args, strings.TrimSpace(spec.BootID))
	}
	if spec.StartAt != "" {
		clauses = append(clauses, logTimestampExpr+" >= julianday(?)")
		args = append(args, strings.TrimSpace(spec.StartAt))
	}
	if spec.EndAt != "" {
		clauses = append(clauses, logTimestampExpr+" <= julianday(?)")
		args = append(args, strings.TrimSpace(spec.EndAt))
	}
	return clauses, args, nil
}

func normalizeFilterValues(single string, values []string, lower bool) []string {
	normalized := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	for _, value := range append([]string{single}, values...) {
		item := strings.TrimSpace(value)
		if lower {
			item = strings.ToLower(item)
		}
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}

func appendStringSetClause(clauses []string, args []any, column string, values []string) ([]string, []any) {
	if len(values) == 1 {
		clauses = append(clauses, column+" = ?")
		args = append(args, values[0])
		return clauses, args
	}

	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	clauses = append(clauses, column+" IN ("+strings.Join(placeholders, ", ")+")")
	return clauses, args
}

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

func encodeLogCursor(cursor logCursor) string {
	cursor.Version = 1
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeLogCursor(raw string) (*logCursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: decode cursor: %v", ErrInvalidCursor, err)
	}

	var cursor logCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, fmt.Errorf("%w: decode cursor json: %v", ErrInvalidCursor, err)
	}
	if cursor.RowID <= 0 || strings.TrimSpace(cursor.Timestamp) == "" {
		return nil, fmt.Errorf("%w: cursor payload is incomplete", ErrInvalidCursor)
	}

	return &cursor, nil
}
