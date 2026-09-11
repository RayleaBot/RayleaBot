package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Repository struct {
	read   *sql.DB
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

func NewRepository(store *storage.Store) (*Repository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &Repository{
		read:   store.Read,
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
	}, nil
}

func (r *Repository) SaveSummary(ctx context.Context, summary logging.Summary) error {
	summary = logging.NormalizeSummary(summary)
	detailsJSON, err := logging.EncodeJSON(summary.Details)
	if err != nil {
		return fmt.Errorf("encode management log details: %w", err)
	}

	if err := r.writeQ.InsertLogSummary(ctx, sqlcgen.InsertLogSummaryParams{
		LogID:       summary.LogID,
		BootID:      summary.BootID,
		Ts:          summary.Timestamp,
		Level:       strings.ToLower(strings.TrimSpace(summary.Level)),
		Source:      strings.TrimSpace(summary.Source),
		Message:     strings.TrimSpace(summary.Message),
		PluginID:    strings.TrimSpace(summary.PluginID),
		RequestID:   strings.TrimSpace(summary.RequestID),
		DetailsJson: detailsJSON,
	}); err != nil {
		return fmt.Errorf("insert management log summary: %w", err)
	}
	return nil
}

func (r *Repository) ListSummaries(ctx context.Context, query logging.Query) ([]logging.Summary, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	clauses, args := buildLogFilterClauses(filterSpec{
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
	defer func(release func() error) { _ = release() }(rows.Close)

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

func (r *Repository) ListPage(ctx context.Context, query logging.PageQuery) (logging.PageResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	direction := query.Direction
	if direction == "" {
		direction = logging.PageDirectionOlder
	}

	clauses, args := buildLogFilterClauses(filterSpecFromPageQuery(query))

	cursor, err := decodeLogCursor(query.Cursor)
	if err != nil {
		return logging.PageResult{}, err
	}
	if cursor == nil {
		direction = logging.PageDirectionOlder
	}
	if cursor != nil {
		switch direction {
		case logging.PageDirectionOlder:
			clauses = append(clauses, "("+logTimestampExpr+" < julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id < ?))")
			args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.RowID)
		case logging.PageDirectionNewer:
			clauses = append(clauses, "("+logTimestampExpr+" > julianday(?) OR ("+logTimestampExpr+" = julianday(?) AND id > ?))")
			args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.RowID)
		default:
			return logging.PageResult{}, fmt.Errorf("%w: unsupported direction %q", logging.ErrInvalidCursor, direction)
		}
	}

	orderClause := "ORDER BY " + logTimestampExpr + " DESC, id DESC"
	if direction == logging.PageDirectionNewer {
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
		return logging.PageResult{}, fmt.Errorf("query management log page: %w", err)
	}
	defer func(release func() error) { _ = release() }(rows.Close)

	entries := make([]pagedSummary, 0, limit+1)
	for rows.Next() {
		entry, err := scanPagedSummary(rows)
		if err != nil {
			return logging.PageResult{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return logging.PageResult{}, fmt.Errorf("iterate management log page: %w", err)
	}

	if direction == logging.PageDirectionNewer {
		for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	result := logging.PageResult{
		Items: make([]logging.Summary, 0, len(entries)),
		Page: logging.PageInfo{
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

	hasOlder, err := r.hasRows(ctx, filterSpecFromPageQuery(query), logBoundaryOlder, oldest.marker())
	if err != nil {
		return logging.PageResult{}, err
	}
	hasNewer, err := r.hasRows(ctx, filterSpecFromPageQuery(query), logBoundaryNewer, newest.marker())
	if err != nil {
		return logging.PageResult{}, err
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

func (r *Repository) GetSummary(ctx context.Context, logID string) (logging.Summary, error) {
	item, err := r.readQ.GetLogSummary(ctx, strings.TrimSpace(logID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return logging.Summary{}, logging.ErrLogNotFound
		}
		return logging.Summary{}, fmt.Errorf("query management log detail: %w", err)
	}

	details, err := logging.DecodeJSON(item.DetailsJson)
	if err != nil {
		return logging.Summary{}, fmt.Errorf("decode management log detail %s: %w", item.LogID, err)
	}

	return logging.NormalizeSummary(logging.Summary{
		BootID:    item.BootID,
		LogID:     item.LogID,
		Timestamp: item.Ts,
		Level:     item.Level,
		Source:    item.Source,
		Message:   item.Message,
		PluginID:  item.PluginID,
		RequestID: item.RequestID,
		Details:   details,
	}), nil
}

func (r *Repository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}

	if err := r.writeQ.PruneLogsBefore(ctx, cutoff.UTC().Format(time.RFC3339)); err != nil {
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

const logTimestampExpr = "julianday(ts)"

func buildLogFilterClauses(spec filterSpec) ([]string, []any) {
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
		sources := logging.SourcesForProtocol(spec.Protocol)
		if len(sources) == 0 {
			return []string{"1 = 0"}, args
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
	return clauses, args
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

type logCursor struct {
	Version   int    `json:"v"`
	RowID     int64  `json:"row_id"`
	Timestamp string `json:"ts"`
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
		return nil, fmt.Errorf("%w: decode cursor: %v", logging.ErrInvalidCursor, err)
	}

	var cursor logCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, fmt.Errorf("%w: decode cursor json: %v", logging.ErrInvalidCursor, err)
	}
	if cursor.RowID <= 0 || strings.TrimSpace(cursor.Timestamp) == "" {
		return nil, fmt.Errorf("%w: cursor payload is incomplete", logging.ErrInvalidCursor)
	}

	return &cursor, nil
}

type pagedSummary struct {
	RowID   int64
	Summary logging.Summary
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
	entry.Summary = logging.NormalizeSummary(entry.Summary)
	return entry, nil
}

func (r *Repository) hasRows(ctx context.Context, spec filterSpec, boundary logBoundary, marker logCursor) (bool, error) {
	clauses, args := buildLogFilterClauses(spec)
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

func filterSpecFromPageQuery(q logging.PageQuery) filterSpec {
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
