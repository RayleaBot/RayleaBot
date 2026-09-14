// Package secrets defines credential storage.
package secrets

import (
	"context"
	"errors"
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

// BatchStore commits a credential replacement or deletion as one transaction.
type BatchStore interface {
	Store
	Apply(context.Context, map[string][]byte, []string) error
}
