package auth

import (
	"context"
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
	return m.BootstrapWithContext(context.Background(), identifier, secret)
}

func (m *Manager) BootstrapWithContext(ctx context.Context, identifier, secret string) (string, Claims, error) {
	ctx = normalizeContext(ctx)
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || secret == "" {
		return "", Claims{}, ErrInvalidToken
	}

	m.mu.Lock()
	if m.bootstrap != nil {
		m.mu.Unlock()
		return "", Claims{}, ErrBootstrapAlreadyInitialized
	}
	m.mu.Unlock()

	now := m.now().UTC()
	canonicalNow := canonicalSessionTimestamp(now)
	secretDigest, err := hashSecret(secret, m.passwordHashParams)
	if err != nil {
		return "", Claims{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bootstrap != nil {
		return "", Claims{}, ErrBootstrapAlreadyInitialized
	}

	bootstrapState := BootstrapState{
		Identifier:    identifier,
		SecretDigest:  secretDigest,
		SigningKey:    append([]byte(nil), m.signingKey...),
		InitializedAt: canonicalNow,
	}
	token, claims, err := m.newTokenClaimsLocked(identifier, now)
	if err != nil {
		return "", Claims{}, err
	}

	removed := m.pruneExpiredLocked(now)
	removed = append(removed, m.recycleOldestSessionsLocked()...)
	if err := m.deleteSessionsLocked(ctx, removed...); err != nil {
		return "", Claims{}, err
	}
	if len(m.sessions) >= m.cfg.MaxSessions {
		return "", Claims{}, ErrSessionLimitReached
	}

	if m.repo != nil {
		if err := m.repo.SaveBootstrap(ctx, bootstrapState, claims); err != nil {
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
	return m.LoginWithContext(context.Background(), identifier, secret)
}

func (m *Manager) LoginWithContext(ctx context.Context, identifier, secret string) (string, Claims, error) {
	ctx = normalizeContext(ctx)
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
	if m.bootstrap.Identifier != identifier {
		return "", Claims{}, ErrInvalidCredentials
	}
	verification := verifySecret(secret, m.bootstrap.SecretDigest)
	if !verification.OK {
		return "", Claims{}, ErrInvalidCredentials
	}
	if verification.Legacy {
		secretDigest, err := hashSecret(secret, m.passwordHashParams)
		if err != nil {
			return "", Claims{}, err
		}
		if m.repo != nil {
			if err := m.repo.UpdateBootstrapSecretDigest(ctx, secretDigest); err != nil {
				return "", Claims{}, fmt.Errorf("upgrade bootstrap secret digest: %w", err)
			}
		}
		m.bootstrap.SecretDigest = append([]byte(nil), secretDigest...)
	}

	return m.issueLocked(ctx, identifier, now)
}

func (m *Manager) newTokenClaimsLocked(subject string, now time.Time) (string, Claims, error) {
	sessionID, err := m.sessionID()
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate session id: %w", err)
	}

	claims := Claims{
		SessionID: sessionID,
		Subject:   subject,
		IssuedAt:  canonicalSessionTimestamp(now),
		ExpiresAt: canonicalSessionTimestamp(now.Add(m.ttl())),
	}

	token, err := m.sign(claims)
	if err != nil {
		return "", Claims{}, err
	}

	return token, claims, nil
}
