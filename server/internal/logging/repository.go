package logging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
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
