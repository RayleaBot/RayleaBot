package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
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

	token, claims, err := m.issueLocked(identifier, now)
	if err != nil {
		return "", Claims{}, err
	}

	m.bootstrap = &bootstrapCredentials{
		Identifier:    identifier,
		SecretDigest:  digestSecret(secret),
		InitializedAt: now,
	}

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
