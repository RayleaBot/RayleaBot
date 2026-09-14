package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	legacySecretKeyName      = "platform.secret_encryption_key"
	legacySealedSecretPrefix = "raylea-secret:v1:"
)

// decryptLegacySecrets rewrites 000002 secrets as plaintext and drops the AES
// key, which was stored in the same table as the ciphertext. A value that cannot
// be opened fails the step, leaving the key and every ciphertext in place.
func decryptLegacySecrets(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT key, value FROM secret_store")
	if err != nil {
		return err
	}
	values := map[string][]byte{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			_ = rows.Close()
			return err
		}
		values[key] = value
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for key, value := range values {
		if key == legacySecretKeyName || !strings.HasPrefix(string(value), legacySealedSecretPrefix) {
			continue
		}
		plaintext, err := openLegacySecret(values[legacySecretKeyName], string(value))
		if err != nil {
			return fmt.Errorf("decrypt secret %s: %w", key, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE secret_store SET value = ? WHERE key = ?", plaintext, key); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM secret_store WHERE key = ?", legacySecretKeyName)
	return err
}

func openLegacySecret(key []byte, envelope string) ([]byte, error) {
	parts := strings.Split(strings.TrimPrefix(envelope, legacySealedSecretPrefix), ":")
	if len(key) != 32 || len(parts) != 2 {
		return nil, errors.New("invalid encrypted secret envelope")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid encrypted secret nonce")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
