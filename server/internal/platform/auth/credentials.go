package auth

import (
	"context"
	"crypto/hmac"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidCredentialInput = errors.New("invalid credential update input")

func ValidateCredentialUpdate(currentSecret, newSecret, newIdentifier string) error {
	if currentSecret == "" || utf8.RuneCountInString(newSecret) < 8 || utf8.RuneCountInString(newSecret) > 1024 || utf8.RuneCountInString(newIdentifier) > 128 {
		return ErrInvalidCredentialInput
	}
	return nil
}

// UpdateCredentialsWithContext verifies the current password before atomically
// replacing the credential source and revoking every existing session.
func (m *Manager) UpdateCredentialsWithContext(ctx context.Context, claims Claims, currentSecret, newSecret, newIdentifier string) error {
	ctx = normalizeContext(ctx)
	if err := ValidateCredentialUpdate(currentSecret, newSecret, newIdentifier); err != nil {
		return err
	}
	newIdentifier = strings.TrimSpace(newIdentifier)

	m.stateMu.RLock()
	if !m.validAccountSessionLocked(claims) {
		m.stateMu.RUnlock()
		return ErrInvalidToken
	}
	identifier := m.bootstrap.Identifier
	digest := append([]byte(nil), m.bootstrap.SecretDigest...)
	m.stateMu.RUnlock()

	release, err := m.acquirePasswordSlot(ctx)
	if err != nil {
		return err
	}
	if !verifySecret(currentSecret, digest) {
		release()
		return ErrInvalidCredentials
	}
	newDigest, err := hashSecret(newSecret, m.passwordHashParams)
	release()
	if err != nil {
		return err
	}

	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()
	m.stateMu.RLock()
	validSession := m.validAccountSessionLocked(claims)
	unchanged := m.bootstrap != nil && m.bootstrap.Identifier == identifier && hmac.Equal(m.bootstrap.SecretDigest, digest)
	m.stateMu.RUnlock()
	if !validSession {
		return ErrInvalidToken
	}
	if !unchanged {
		return ErrInvalidCredentials
	}
	if newIdentifier == "" {
		newIdentifier = identifier
	}
	if m.repo != nil {
		if err := m.repo.UpdateCredentials(ctx, newIdentifier, newDigest); err != nil {
			return err
		}
	}

	m.stateMu.Lock()
	m.bootstrap.Identifier = newIdentifier
	m.bootstrap.SecretDigest = append([]byte(nil), newDigest...)
	clear(m.sessions)
	close(m.credentialsChanged)
	m.credentialsChanged = make(chan struct{})
	m.stateMu.Unlock()
	return nil
}

// CredentialsChanged returns the current credential generation's revocation
// signal. Long-lived clients capture it before validating their session.
func (m *Manager) CredentialsChanged() <-chan struct{} {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.credentialsChanged
}

// Caller holds stateMu. Check again after password hashing to reject a session
// revoked, expired or replaced while the expensive verification was in flight.
func (m *Manager) validAccountSessionLocked(claims Claims) bool {
	stored, ok := m.sessions[claims.SessionID]
	now := m.now().UTC()
	return ok && m.bootstrap != nil && stored.Subject == m.bootstrap.Identifier && stored.Subject == claims.Subject &&
		stored.IssuedAt.Equal(claims.IssuedAt) && now.Before(stored.ExpiresAt) && now.Before(stored.IssuedAt.Add(m.absoluteTTL()))
}
