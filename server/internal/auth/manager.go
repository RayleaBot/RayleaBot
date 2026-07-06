package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	ErrInvalidToken        = errors.New("invalid session token")
	ErrExpiredToken        = errors.New("expired session token")
	ErrSessionLimitReached = errors.New("maximum active sessions reached")
)

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

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
	now                func() time.Time
	signingKey         []byte
	sessionID          func() (string, error)
	repo               Repository
	passwordHashParams passwordHashParams
}

type Manager struct {
	cfg Config

	now                func() time.Time
	signingKey         []byte
	sessionID          func() (string, error)
	repo               Repository
	passwordHashParams passwordHashParams

	mu        sync.Mutex
	sessions  map[string]Claims
	bootstrap *bootstrapCredentials
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

func withPasswordHashParams(params passwordHashParams) Option {
	return func(options *managerOptions) error {
		if err := params.validateForHashing(); err != nil {
			return err
		}
		options.passwordHashParams = params
		return nil
	}
}

func NewManager(cfg Config, opts ...Option) (*Manager, error) {
	return NewManagerWithContext(context.Background(), cfg, opts...)
}

func NewManagerWithContext(ctx context.Context, cfg Config, opts ...Option) (*Manager, error) {
	ctx = normalizeContext(ctx)
	if cfg.SessionTTLDays <= 0 {
		return nil, fmt.Errorf("session_ttl_days must be positive")
	}
	if cfg.MaxSessions <= 0 {
		return nil, fmt.Errorf("max_sessions must be positive")
	}

	options := managerOptions{
		now:                time.Now,
		passwordHashParams: defaultPasswordHashParams,
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
		cfg:                cfg,
		now:                options.now,
		signingKey:         options.signingKey,
		sessionID:          options.sessionID,
		repo:               options.repo,
		passwordHashParams: options.passwordHashParams,
		sessions:           make(map[string]Claims),
	}

	if manager.repo != nil {
		if err := manager.hydrate(ctx); err != nil {
			return nil, err
		}
	}

	return manager, nil
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
		claims.IssuedAt = canonicalSessionTimestamp(claims.IssuedAt)
		claims.ExpiresAt = canonicalSessionTimestamp(claims.ExpiresAt)
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

type LoginFailureRecorder interface {
	IsLimited(string, int, time.Duration) bool
	RecordFailure(string, int, time.Duration)
	Reset(string)
}

type LoginFailureTracker struct {
	now func() time.Time

	mu      sync.Mutex
	entries map[string][]time.Time
}

func NewLoginFailureTracker(now func() time.Time) *LoginFailureTracker {
	if now == nil {
		now = time.Now
	}
	return &LoginFailureTracker{
		now:     now,
		entries: make(map[string][]time.Time),
	}
}

func (t *LoginFailureTracker) IsLimited(source string, limit int, window time.Duration) bool {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.prunedLocked(source, window)
	return len(entries) >= limit
}

func (t *LoginFailureTracker) RecordFailure(source string, limit int, window time.Duration) {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := append(t.prunedLocked(source, window), t.now().UTC())
	t.entries[source] = entries
}

func (t *LoginFailureTracker) Reset(source string) {
	if t == nil || source == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, source)
}

func (t *LoginFailureTracker) prunedLocked(source string, window time.Duration) []time.Time {
	if t == nil || source == "" {
		return nil
	}

	entries := t.entries[source]
	if len(entries) == 0 {
		delete(t.entries, source)
		return nil
	}

	cutoff := t.now().UTC().Add(-window)
	filtered := entries[:0]
	for _, entry := range entries {
		if !entry.Before(cutoff) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(t.entries, source)
		return nil
	}

	t.entries[source] = filtered
	return filtered
}

func loginFailureTrackingEnabled(source string, limit int, window time.Duration) bool {
	return source != "" && limit > 0 && window > 0
}
