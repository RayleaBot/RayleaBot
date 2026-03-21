// Package secrets provides a unified interface for storing and retrieving
// sensitive credentials. The default implementation uses SQLite, keeping all
// secrets local to the host machine.
package secrets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rayleabot/server/internal/storage"
)

// ErrNotFound is returned when a requested secret key does not exist.
var ErrNotFound = errors.New("secret not found")

// Store defines the interface for secret storage operations.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
}

// SQLiteStore implements Store using the platform SQLite database.
type SQLiteStore struct {
	read  *sql.DB
	write *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed secret store.
func NewSQLiteStore(store *storage.Store) (*SQLiteStore, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteStore{
		read:  store.Read,
		write: store.Write,
	}, nil
}

// Get retrieves a secret by key. Returns ErrNotFound if the key does not exist.
func (s *SQLiteStore) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.read.QueryRowContext(ctx,
		`SELECT value FROM secret_store WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get secret %q: %w", key, err)
	}
	return append([]byte(nil), value...), nil
}

// Set stores or updates a secret. The value is stored as a raw byte blob.
func (s *SQLiteStore) Set(ctx context.Context, key string, value []byte) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.write.ExecContext(ctx,
		`INSERT INTO secret_store (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at`,
		key, value, now, now,
	); err != nil {
		return fmt.Errorf("set secret %q: %w", key, err)
	}
	return nil
}

// Delete removes a secret by key. No error is returned if the key does not exist.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	if _, err := s.write.ExecContext(ctx,
		`DELETE FROM secret_store WHERE key = ?`, key,
	); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}

// List returns all stored secret keys (not values).
func (s *SQLiteStore) List(ctx context.Context) ([]string, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT key FROM secret_store ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan secret key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
