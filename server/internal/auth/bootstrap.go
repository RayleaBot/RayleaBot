package auth

import (
	"context"
	"crypto/hmac"
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

	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()
	m.stateMu.RLock()
	initialized := m.bootstrap != nil
	m.stateMu.RUnlock()
	if initialized {
		return "", Claims{}, ErrBootstrapAlreadyInitialized
	}

	release, err := m.acquirePasswordSlot(ctx)
	if err != nil {
		return "", Claims{}, err
	}
	secretDigest, err := hashSecret(secret, m.passwordHashParams)
	release()
	if err != nil {
		return "", Claims{}, err
	}

	now := m.now().UTC()
	bootstrapState := BootstrapState{
		Identifier:    identifier,
		SecretDigest:  secretDigest,
		SigningKey:    append([]byte(nil), m.signingKey...),
		InitializedAt: canonicalSessionTimestamp(now),
	}
	token, claims, err := m.newTokenClaims(identifier, now)
	if err != nil {
		return "", Claims{}, err
	}

	m.stateMu.RLock()
	removed := m.sessionIDsToRemoveLocked(now)
	m.stateMu.RUnlock()
	if err := m.deleteSessions(ctx, removed...); err != nil {
		return "", Claims{}, err
	}
	if m.repo != nil {
		if err := m.repo.SaveBootstrap(ctx, bootstrapState, claims); err != nil {
			if errors.Is(err, ErrBootstrapAlreadyInitialized) {
				return "", Claims{}, ErrBootstrapAlreadyInitialized
			}
			return "", Claims{}, err
		}
	}

	m.stateMu.Lock()
	if m.bootstrap != nil {
		m.stateMu.Unlock()
		return "", Claims{}, ErrBootstrapAlreadyInitialized
	}
	for _, sessionID := range removed {
		delete(m.sessions, sessionID)
	}
	m.bootstrap = &bootstrapCredentials{
		Identifier:    bootstrapState.Identifier,
		SecretDigest:  append([]byte(nil), bootstrapState.SecretDigest...),
		InitializedAt: bootstrapState.InitializedAt,
	}
	m.sessions[claims.SessionID] = claims
	m.stateMu.Unlock()
	return token, claims, nil
}

func (m *Manager) IsBootstrapped() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
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

	m.stateMu.RLock()
	if m.bootstrap == nil || m.bootstrap.Identifier != identifier {
		m.stateMu.RUnlock()
		return "", Claims{}, ErrInvalidCredentials
	}
	secretDigest := append([]byte(nil), m.bootstrap.SecretDigest...)
	m.stateMu.RUnlock()

	release, err := m.acquirePasswordSlot(ctx)
	if err != nil {
		return "", Claims{}, err
	}
	verification := verifySecret(secret, secretDigest)
	var upgradedDigest []byte
	if verification.OK && verification.Legacy {
		upgradedDigest, err = hashSecret(secret, m.passwordHashParams)
	}
	release()
	if err != nil {
		return "", Claims{}, err
	}
	if !verification.OK {
		return "", Claims{}, ErrInvalidCredentials
	}

	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()
	m.stateMu.RLock()
	current := m.bootstrap
	unchanged := current != nil && current.Identifier == identifier && hmac.Equal(current.SecretDigest, secretDigest)
	m.stateMu.RUnlock()
	if !unchanged {
		return "", Claims{}, ErrInvalidCredentials
	}
	if len(upgradedDigest) > 0 {
		if m.repo != nil {
			if err := m.repo.UpdateBootstrapSecretDigest(ctx, upgradedDigest); err != nil {
				return "", Claims{}, fmt.Errorf("upgrade bootstrap secret digest: %w", err)
			}
		}
		m.stateMu.Lock()
		m.bootstrap.SecretDigest = append([]byte(nil), upgradedDigest...)
		m.stateMu.Unlock()
	}

	return m.issueSerialized(ctx, identifier, m.now().UTC())
}

func (m *Manager) acquirePasswordSlot(ctx context.Context) (func(), error) {
	select {
	case m.passwordSemaphore <- struct{}{}:
		return func() { <-m.passwordSemaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Manager) newTokenClaims(subject string, now time.Time) (string, Claims, error) {
	sessionID, err := m.sessionID()
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate session id: %w", err)
	}

	issuedAt := canonicalSessionTimestamp(now)
	expiresAt := canonicalSessionTimestamp(now.Add(m.ttl()))
	absoluteExpiry := canonicalSessionTimestamp(issuedAt.Add(m.absoluteTTL()))
	if expiresAt.After(absoluteExpiry) {
		expiresAt = absoluteExpiry
	}
	claims := Claims{SessionID: sessionID, Subject: subject, IssuedAt: issuedAt, ExpiresAt: expiresAt}
	token, err := m.sign(claims)
	if err != nil {
		return "", Claims{}, err
	}
	return token, claims, nil
}
