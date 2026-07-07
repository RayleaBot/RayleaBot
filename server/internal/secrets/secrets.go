// Package secrets provides a unified interface for storing and retrieving
// sensitive credentials. The default implementation uses SQLite, keeping all
// secrets local to the host machine.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

// ErrNotFound is returned when a requested secret key does not exist.
var ErrNotFound = errors.New("secret not found")

const (
	encryptionKeyName = "platform.secret_encryption_key"
	sealedPrefix      = "raylea-secret:v1:"
)

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

// SealString encrypts a plaintext value before it is written to the shared
// secret table. Values are encoded as a versioned ASCII envelope.
func SealString(ctx context.Context, store Store, plaintext string) ([]byte, error) {
	if store == nil {
		return nil, errors.New("secret store is required")
	}
	key, err := encryptionKey(ctx, store)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("create secret nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	envelope := sealedPrefix +
		base64.RawURLEncoding.EncodeToString(nonce) +
		":" +
		base64.RawURLEncoding.EncodeToString(ciphertext)
	return []byte(envelope), nil
}

// OpenString decrypts a versioned encrypted value.
func OpenString(ctx context.Context, store Store, stored []byte) (string, error) {
	text := string(stored)
	if !strings.HasPrefix(text, sealedPrefix) {
		return "", errors.New("invalid encrypted secret envelope")
	}
	if store == nil {
		return "", errors.New("secret store is required")
	}
	key, err := encryptionKey(ctx, store)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimPrefix(text, sealedPrefix), ":")
	if len(parts) != 2 {
		return "", errors.New("invalid encrypted secret envelope")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode secret nonce: %w", err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode secret ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func encryptionKey(ctx context.Context, store Store) ([]byte, error) {
	key, err := store.Get(ctx, encryptionKeyName)
	if err == nil {
		if len(key) != 32 {
			return nil, errors.New("secret encryption key has invalid length")
		}
		return key, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("create secret encryption key: %w", err)
	}
	if err := store.Set(ctx, encryptionKeyName, key); err != nil {
		return nil, err
	}
	return key, nil
}
