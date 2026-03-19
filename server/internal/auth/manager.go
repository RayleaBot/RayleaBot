package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidToken        = errors.New("invalid session token")
	ErrExpiredToken        = errors.New("expired session token")
	ErrSessionLimitReached = errors.New("maximum active sessions reached")
)

type Config struct {
	SessionTTLDays int
	SlidingRenewal bool
	MaxSessions    int
}

type Claims struct {
	SessionID string
	Subject   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type Option func(*managerOptions) error

type managerOptions struct {
	now        func() time.Time
	signingKey []byte
	sessionID  func() (string, error)
}

type Manager struct {
	cfg Config

	now        func() time.Time
	signingKey []byte
	sessionID  func() (string, error)

	mu       sync.Mutex
	sessions map[string]Claims
}

type tokenClaims struct {
	Version   int    `json:"v"`
	SessionID string `json:"sid"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func WithClock(now func() time.Time) Option {
	return func(options *managerOptions) error {
		if now == nil {
			return errors.New("clock is required")
		}
		options.now = now
		return nil
	}
}

func WithSigningKey(signingKey []byte) Option {
	return func(options *managerOptions) error {
		if len(signingKey) == 0 {
			return errors.New("signing key is required")
		}
		options.signingKey = append([]byte(nil), signingKey...)
		return nil
	}
}

func WithSessionIDGenerator(generator func() (string, error)) Option {
	return func(options *managerOptions) error {
		if generator == nil {
			return errors.New("session id generator is required")
		}
		options.sessionID = generator
		return nil
	}
}

func NewManager(cfg Config, opts ...Option) (*Manager, error) {
	if cfg.SessionTTLDays <= 0 {
		return nil, fmt.Errorf("session_ttl_days must be positive")
	}
	if cfg.MaxSessions <= 0 {
		return nil, fmt.Errorf("max_sessions must be positive")
	}

	options := managerOptions{
		now: time.Now,
		sessionID: func() (string, error) {
			return randomTokenSegment(16)
		},
	}

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	if len(options.signingKey) == 0 {
		signingKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, signingKey); err != nil {
			return nil, fmt.Errorf("generate session signing key: %w", err)
		}
		options.signingKey = signingKey
	}

	return &Manager{
		cfg:        cfg,
		now:        options.now,
		signingKey: options.signingKey,
		sessionID:  options.sessionID,
		sessions:   make(map[string]Claims),
	}, nil
}

func (m *Manager) Issue(subject string) (string, Claims, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", Claims{}, fmt.Errorf("subject is required")
	}

	now := m.now().UTC()
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

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)
	if len(m.sessions) >= m.cfg.MaxSessions {
		return "", Claims{}, ErrSessionLimitReached
	}

	m.sessions[claims.SessionID] = claims
	return token, claims, nil
}

func (m *Manager) Validate(token string) (Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Claims{}, ErrInvalidToken
	}

	parsed, err := m.verify(token)
	if err != nil {
		return Claims{}, err
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.sessions[parsed.SessionID]
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	if stored.Subject != parsed.Subject || !stored.IssuedAt.Equal(parsed.IssuedAt) {
		return Claims{}, ErrInvalidToken
	}
	if !now.Before(stored.ExpiresAt) {
		delete(m.sessions, stored.SessionID)
		return Claims{}, ErrExpiredToken
	}

	if m.cfg.SlidingRenewal {
		stored.ExpiresAt = now.Add(m.ttl())
		m.sessions[stored.SessionID] = stored
	}

	return stored, nil
}

func (m *Manager) ttl() time.Duration {
	return time.Duration(m.cfg.SessionTTLDays) * 24 * time.Hour
}

func (m *Manager) pruneExpiredLocked(now time.Time) {
	for sessionID, claims := range m.sessions {
		if !now.Before(claims.ExpiresAt) {
			delete(m.sessions, sessionID)
		}
	}
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
		IssuedAt:  time.Unix(payload.IssuedAt, 0).UTC(),
		ExpiresAt: time.Unix(payload.ExpiresAt, 0).UTC(),
	}, nil
}

func randomTokenSegment(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
