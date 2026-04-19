package permission

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type WhitelistEntry struct {
	ID        int64
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type WhitelistRepository interface {
	IsWhitelisted(ctx context.Context, entryType, targetID string) (bool, error)
	Get(ctx context.Context, entryType, targetID string) (WhitelistEntry, error)
	Add(ctx context.Context, entryType, targetID, reason string) error
	Remove(ctx context.Context, entryType, targetID string) error
	List(ctx context.Context, entryType string) ([]WhitelistEntry, error)
}

type WhitelistStateRepository interface {
	Enabled(ctx context.Context) (bool, error)
	SetEnabled(ctx context.Context, enabled bool) error
}

type SQLiteWhitelistRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteWhitelistRepository(read, write *sql.DB) *SQLiteWhitelistRepository {
	return &SQLiteWhitelistRepository{read: read, write: write}
}

func (r *SQLiteWhitelistRepository) IsWhitelisted(ctx context.Context, entryType, targetID string) (bool, error) {
	var count int
	err := r.read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM whitelist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteWhitelistRepository) Get(ctx context.Context, entryType, targetID string) (WhitelistEntry, error) {
	var entry WhitelistEntry
	err := r.read.QueryRowContext(ctx,
		`SELECT id, entry_type, target_id, reason, created_at
		 FROM whitelist_entries
		 WHERE entry_type = ? AND target_id = ?`,
		entryType, targetID,
	).Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WhitelistEntry{}, ErrGovernanceEntryNotFound
	}
	if err != nil {
		return WhitelistEntry{}, err
	}
	return entry, nil
}

func (r *SQLiteWhitelistRepository) Add(ctx context.Context, entryType, targetID, reason string) error {
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO whitelist_entries (entry_type, target_id, reason, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (entry_type, target_id) DO UPDATE SET reason = excluded.reason`,
		entryType, targetID, reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteWhitelistRepository) Remove(ctx context.Context, entryType, targetID string) error {
	result, err := r.write.ExecContext(ctx,
		"DELETE FROM whitelist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *SQLiteWhitelistRepository) List(ctx context.Context, entryType string) ([]WhitelistEntry, error) {
	rows, err := r.read.QueryContext(ctx,
		"SELECT id, entry_type, target_id, reason, created_at FROM whitelist_entries WHERE entry_type = ? ORDER BY created_at DESC",
		entryType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WhitelistEntry
	for rows.Next() {
		var entry WhitelistEntry
		if err := rows.Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

type SQLiteWhitelistStateRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteWhitelistStateRepository(read, write *sql.DB) *SQLiteWhitelistStateRepository {
	return &SQLiteWhitelistStateRepository{read: read, write: write}
}

func (r *SQLiteWhitelistStateRepository) Enabled(ctx context.Context) (bool, error) {
	var enabled int
	err := r.read.QueryRowContext(ctx,
		"SELECT enabled FROM whitelist_state WHERE singleton_id = 1",
	).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled == 1, nil
}

func (r *SQLiteWhitelistStateRepository) SetEnabled(ctx context.Context, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO whitelist_state (singleton_id, enabled, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT (singleton_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
		value, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
