package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	encryptionKeyName = "platform.secret_encryption_key"
	sealedPrefix      = "raylea-secret:v1:"
)

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

// OpenString decrypts a versioned encrypted value. Plain legacy values are
// returned as-is so existing local deployments remain readable.
func OpenString(ctx context.Context, store Store, stored []byte) (string, error) {
	text := string(stored)
	if !strings.HasPrefix(text, sealedPrefix) {
		return text, nil
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
