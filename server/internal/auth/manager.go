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
