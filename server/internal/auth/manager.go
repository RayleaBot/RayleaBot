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
	repo       Repository
}

type Manager struct {
	cfg Config

	now        func() time.Time
	signingKey []byte
	sessionID  func() (string, error)
	repo       Repository

	mu        sync.Mutex
	sessions  map[string]Claims
	bootstrap *bootstrapCredentials
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

func WithRepository(repo Repository) Option {
	return func(options *managerOptions) error {
		if repo == nil {
			return errors.New("repository is required")
		}
		options.repo = repo
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

	manager := &Manager{
		cfg:        cfg,
		now:        options.now,
		signingKey: options.signingKey,
		sessionID:  options.sessionID,
		repo:       options.repo,
		sessions:   make(map[string]Claims),
	}

	if manager.repo != nil {
		if err := manager.hydrate(context.Background()); err != nil {
			return nil, err
		}
	}

	return manager, nil
}

func (m *Manager) Issue(subject string) (string, Claims, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", Claims{}, fmt.Errorf("subject is required")
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.issueLocked(subject, now)
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
		if err := m.deleteSessionsLocked(context.Background(), stored.SessionID); err != nil {
			return Claims{}, err
		}
		return Claims{}, ErrExpiredToken
	}

	if m.cfg.SlidingRenewal {
		stored.ExpiresAt = now.Add(m.ttl())
		if err := m.saveSessionLocked(context.Background(), stored); err != nil {
			return Claims{}, err
		}
		m.sessions[stored.SessionID] = stored
	}

	return stored, nil
}

func (m *Manager) ttl() time.Duration {
	return time.Duration(m.cfg.SessionTTLDays) * 24 * time.Hour
}

func (m *Manager) pruneExpiredLocked(now time.Time) []string {
	var removed []string
	for sessionID, claims := range m.sessions {
		if !now.Before(claims.ExpiresAt) {
			delete(m.sessions, sessionID)
			removed = append(removed, sessionID)
		}
	}
	return removed
}

func (m *Manager) issueLocked(subject string, now time.Time) (string, Claims, error) {
	token, claims, err := m.newTokenClaimsLocked(subject, now)
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
	if err := m.saveSessionLocked(context.Background(), claims); err != nil {
		return "", Claims{}, err
	}

	m.sessions[claims.SessionID] = claims
	return token, claims, nil
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

func (m *Manager) hydrate(ctx context.Context) error {
	state, err := m.repo.LoadBootstrap(ctx)
	if err != nil {
		return fmt.Errorf("load bootstrap state: %w", err)
	}
	if state != nil {
		m.bootstrap = &bootstrapCredentials{
			Identifier:    state.Identifier,
			SecretDigest:  append([]byte(nil), state.SecretDigest...),
			InitializedAt: state.InitializedAt.UTC(),
		}
		m.signingKey = append([]byte(nil), state.SigningKey...)
	}

	sessions, err := m.repo.LoadSessions(ctx)
	if err != nil {
		return fmt.Errorf("load admin sessions: %w", err)
	}

	now := m.now().UTC()
	var expired []string
	for _, claims := range sessions {
		if now.Before(claims.ExpiresAt) {
			m.sessions[claims.SessionID] = claims
			continue
		}
		expired = append(expired, claims.SessionID)
	}
	if err := m.deleteSessionsLocked(ctx, expired...); err != nil {
		return err
	}

	return nil
}

func (m *Manager) saveSessionLocked(ctx context.Context, claims Claims) error {
	if m.repo == nil {
		return nil
	}

	if err := m.repo.SaveSession(ctx, claims); err != nil {
		return fmt.Errorf("persist admin session: %w", err)
	}

	return nil
}

func (m *Manager) deleteSessionsLocked(ctx context.Context, sessionIDs ...string) error {
	if m.repo == nil || len(sessionIDs) == 0 {
		return nil
	}

	if err := m.repo.DeleteSessions(ctx, sessionIDs); err != nil {
		return fmt.Errorf("delete persisted sessions: %w", err)
	}

	return nil
}
