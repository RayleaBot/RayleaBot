package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrBootstrapAlreadyInitialized = errors.New("bootstrap admin already initialized")
var ErrInvalidCredentials = errors.New("invalid bootstrap credentials")

type bootstrapCredentials struct {
	Identifier    string
	SecretDigest  []byte
	InitializedAt time.Time
}

func (m *Manager) Bootstrap(identifier, secret string) (string, Claims, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || secret == "" {
		return "", Claims{}, ErrInvalidToken
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bootstrap != nil {
		return "", Claims{}, ErrBootstrapAlreadyInitialized
	}

	bootstrapState := BootstrapState{
		Identifier:    identifier,
		SecretDigest:  digestSecret(secret),
		SigningKey:    append([]byte(nil), m.signingKey...),
		InitializedAt: now,
	}
	token, claims, err := m.newTokenClaimsLocked(identifier, now)
	if err != nil {
		return "", Claims{}, err
	}

	removed := m.pruneExpiredLocked(now)
	if err := m.deleteSessionsLocked(context.Background(), removed...); err != nil {
		return "", Claims{}, err
	}
	if len(m.sessions) >= m.cfg.MaxSessions {
		return "", Claims{}, ErrSessionLimitReached
	}

	if m.repo != nil {
		if err := m.repo.SaveBootstrap(context.Background(), bootstrapState, claims); err != nil {
			if errors.Is(err, ErrBootstrapAlreadyInitialized) {
				return "", Claims{}, ErrBootstrapAlreadyInitialized
			}
			return "", Claims{}, err
		}
	}

	m.bootstrap = &bootstrapCredentials{
		Identifier:    bootstrapState.Identifier,
		SecretDigest:  append([]byte(nil), bootstrapState.SecretDigest...),
		InitializedAt: bootstrapState.InitializedAt,
	}
	m.sessions[claims.SessionID] = claims

	return token, claims, nil
}

func (m *Manager) IsBootstrapped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.bootstrap != nil
}

func (m *Manager) Login(identifier, secret string) (string, Claims, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || secret == "" {
		return "", Claims{}, ErrInvalidToken
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bootstrap == nil {
		return "", Claims{}, ErrInvalidCredentials
	}
	if m.bootstrap.Identifier != identifier || !secretsEqual(secret, m.bootstrap.SecretDigest) {
		return "", Claims{}, ErrInvalidCredentials
	}

	return m.issueLocked(identifier, now)
}

func digestSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func secretsEqual(secret string, digest []byte) bool {
	return hmac.Equal(digestSecret(secret), digest)
}

func (m *Manager) newTokenClaimsLocked(subject string, now time.Time) (string, Claims, error) {
	sessionID, err := m.sessionID()
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate session id: %w", err)
	}

	claims := Claims{
		SessionID: sessionID,
		Subject:   subject,
		IssuedAt:  now,
		ExpiresAt: now.Add(m.ttl()),
	}

	token, err := m.sign(claims)
	if err != nil {
		return "", Claims{}, err
	}

	return token, claims, nil
}
