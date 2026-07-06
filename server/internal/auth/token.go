package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

const sessionSigningKeySecret = "platform.auth.session_signing_key"

type tokenClaims struct {
	Version   int    `json:"v"`
	SessionID string `json:"sid"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func (m *Manager) sign(claims Claims) (string, error) {
	payload := tokenClaims{
		Version:   1,
		SessionID: claims.SessionID,
		Subject:   claims.Subject,
		IssuedAt:  claims.IssuedAt.UTC().Unix(),
		ExpiresAt: claims.ExpiresAt.UTC().Unix(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal session token payload: %w", err)
	}

	sig := hmac.New(sha256.New, m.signingKey)
	sig.Write(payloadBytes)
	signature := sig.Sum(nil)

	return base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(signature), nil
}

func (m *Manager) verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	sig := hmac.New(sha256.New, m.signingKey)
	sig.Write(payloadBytes)
	if !hmac.Equal(signature, sig.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}

	var payload tokenClaims
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if payload.Version != 1 || payload.SessionID == "" || strings.TrimSpace(payload.Subject) == "" {
		return Claims{}, ErrInvalidToken
	}

	return Claims{
		SessionID: payload.SessionID,
		Subject:   payload.Subject,
		IssuedAt:  canonicalSessionTimestamp(time.Unix(payload.IssuedAt, 0)),
		ExpiresAt: canonicalSessionTimestamp(time.Unix(payload.ExpiresAt, 0)),
	}, nil
}

func randomTokenSegment(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func EnsureSessionSigningKey(ctx context.Context, secretStore secrets.Store) ([]byte, bool, error) {
	if secretStore == nil {
		return nil, false, errors.New("secret store is required")
	}

	signingKey, err := secretStore.Get(ctx, sessionSigningKeySecret)
	switch {
	case err == nil:
		if len(signingKey) == 0 {
			return nil, false, fmt.Errorf("secret %q is empty", sessionSigningKeySecret)
		}
		return signingKey, false, nil
	case !errors.Is(err, secrets.ErrNotFound):
		return nil, false, fmt.Errorf("load session signing key: %w", err)
	}

	signingKey = make([]byte, 32)
	if _, err := rand.Read(signingKey); err != nil {
		return nil, false, fmt.Errorf("generate session signing key: %w", err)
	}
	if err := secretStore.Set(ctx, sessionSigningKeySecret, signingKey); err != nil {
		return nil, false, fmt.Errorf("persist session signing key: %w", err)
	}

	return signingKey, true, nil
}
