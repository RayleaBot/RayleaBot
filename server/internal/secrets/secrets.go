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

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
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
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

// NewSQLiteStore creates a new SQLite-backed secret store.
func NewSQLiteStore(store *storage.Store) (*SQLiteStore, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteStore{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
	}, nil
}

// Get retrieves a secret by key. Returns ErrNotFound if the key does not exist.
func (s *SQLiteStore) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.readQ.GetSecret(ctx, key)
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
	if err := s.writeQ.UpsertSecret(ctx, sqlcgen.UpsertSecretParams{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("set secret %q: %w", key, err)
	}
	return nil
}

// Delete removes a secret by key. No error is returned if the key does not exist.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	if err := s.writeQ.DeleteSecret(ctx, key); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}

// List returns all stored secret keys (not values).
func (s *SQLiteStore) List(ctx context.Context) ([]string, error) {
	keys, err := s.readQ.ListSecretKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	return keys, nil
}
