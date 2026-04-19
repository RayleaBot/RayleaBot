package permission

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrGovernanceEntryNotFound = errors.New("governance entry not found")

type BlacklistEntry struct {
	ID        int64
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type BlacklistRepository interface {
	IsBlacklisted(ctx context.Context, entryType, targetID string) (bool, error)
	Get(ctx context.Context, entryType, targetID string) (BlacklistEntry, error)
	Add(ctx context.Context, entryType, targetID, reason string) error
	Remove(ctx context.Context, entryType, targetID string) error
	List(ctx context.Context, entryType string) ([]BlacklistEntry, error)
}

type SQLiteBlacklistRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteBlacklistRepository(read, write *sql.DB) *SQLiteBlacklistRepository {
	return &SQLiteBlacklistRepository{read: read, write: write}
}

func (r *SQLiteBlacklistRepository) IsBlacklisted(ctx context.Context, entryType, targetID string) (bool, error) {
	var count int
	err := r.read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM blacklist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteBlacklistRepository) Add(ctx context.Context, entryType, targetID, reason string) error {
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO blacklist_entries (entry_type, target_id, reason, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (entry_type, target_id) DO UPDATE SET reason = excluded.reason`,
		entryType, targetID, reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteBlacklistRepository) Get(ctx context.Context, entryType, targetID string) (BlacklistEntry, error) {
	var entry BlacklistEntry
	err := r.read.QueryRowContext(ctx,
		`SELECT id, entry_type, target_id, reason, created_at
		 FROM blacklist_entries
		 WHERE entry_type = ? AND target_id = ?`,
		entryType, targetID,
	).Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BlacklistEntry{}, ErrGovernanceEntryNotFound
	}
	if err != nil {
		return BlacklistEntry{}, err
	}
	return entry, nil
}

func (r *SQLiteBlacklistRepository) Remove(ctx context.Context, entryType, targetID string) error {
	result, err := r.write.ExecContext(ctx,
		"DELETE FROM blacklist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *SQLiteBlacklistRepository) List(ctx context.Context, entryType string) ([]BlacklistEntry, error) {
	rows, err := r.read.QueryContext(ctx,
		"SELECT id, entry_type, target_id, reason, created_at FROM blacklist_entries WHERE entry_type = ? ORDER BY created_at DESC",
		entryType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlacklistEntry
	for rows.Next() {
		var e BlacklistEntry
		if err := rows.Scan(&e.ID, &e.EntryType, &e.TargetID, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
