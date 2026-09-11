package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

// Store implements Store using the platform SQLite database.
type Store struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	write  *sql.DB
}

// NewStore creates a new SQLite-backed secret store.
func NewStore(store *storage.Store) (*Store, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &Store{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		write:  store.Write,
	}, nil
}

func (s *Store) Apply(ctx context.Context, values map[string][]byte, deleted []string) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin secret update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.writeQ.WithTx(tx)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		if err := q.UpsertSecret(ctx, sqlcgen.UpsertSecretParams{Key: key, Value: value, CreatedAt: now, UpdatedAt: now}); err != nil {
			return fmt.Errorf("update secret batch: %w", err)
		}
	}
	for _, key := range deleted {
		if err := q.DeleteSecret(ctx, key); err != nil {
			return fmt.Errorf("delete secret batch: %w", err)
		}
	}
	return tx.Commit()
}

// Get retrieves a secret by key. Returns secrets.ErrNotFound if the key does not exist.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.readQ.GetSecret(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, secrets.ErrNotFound
		}
		return nil, fmt.Errorf("get secret %q: %w", key, err)
	}
	return append([]byte(nil), value...), nil
}

// Set stores or updates a secret. The value is stored as a raw byte blob.
func (s *Store) Set(ctx context.Context, key string, value []byte) error {
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
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.writeQ.DeleteSecret(ctx, key); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}

// List returns all stored secret keys (not values).
func (s *Store) List(ctx context.Context) ([]string, error) {
	keys, err := s.readQ.ListSecretKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	return keys, nil
}
