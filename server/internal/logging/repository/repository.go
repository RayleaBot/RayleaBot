package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

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

func (r *SQLiteRepository) SaveSummary(ctx context.Context, summary logging.Summary) error {
	summary = logging.NormalizeSummary(summary)
	detailsJSON, err := logdetails.EncodeJSON(summary.Details)
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
