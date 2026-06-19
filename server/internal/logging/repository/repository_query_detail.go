package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
)

func (r *SQLiteRepository) GetSummary(ctx context.Context, logID string) (logging.Summary, error) {
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
			return logging.Summary{}, logging.ErrLogNotFound
		}
		return logging.Summary{}, fmt.Errorf("query management log detail: %w", err)
	}

	details, err := logdetails.DecodeJSON(item.DetailsRaw)
	if err != nil {
		return logging.Summary{}, fmt.Errorf("decode management log detail %s: %w", item.LogID, err)
	}

	return logging.NormalizeSummary(logging.Summary{
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
