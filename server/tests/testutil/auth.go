package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
)

type StubAuthRepository struct {
	LoadBootstrapFn   func(context.Context) (*auth.BootstrapState, error)
	LoadSessionsFn    func(context.Context) ([]auth.Claims, error)
	SaveBootstrapFn   func(context.Context, auth.BootstrapState, auth.Claims) error
	UpdateBootstrapFn func(context.Context, []byte) error
	SaveSessionFn     func(context.Context, auth.Claims) error
	DeleteSessionsFn  func(context.Context, []string) error
}

func (r *StubAuthRepository) LoadBootstrap(ctx context.Context) (*auth.BootstrapState, error) {
	if r != nil && r.LoadBootstrapFn != nil {
		return r.LoadBootstrapFn(ctx)
	}
	return nil, nil
}

func (r *StubAuthRepository) LoadSessions(ctx context.Context) ([]auth.Claims, error) {
	if r != nil && r.LoadSessionsFn != nil {
		return r.LoadSessionsFn(ctx)
	}
	return nil, nil
}

func (r *StubAuthRepository) SaveBootstrap(ctx context.Context, state auth.BootstrapState, claims auth.Claims) error {
	if r != nil && r.SaveBootstrapFn != nil {
		return r.SaveBootstrapFn(ctx, state, claims)
	}
	return nil
}

func (r *StubAuthRepository) SaveSession(ctx context.Context, claims auth.Claims) error {
	if r != nil && r.SaveSessionFn != nil {
		return r.SaveSessionFn(ctx, claims)
	}
	return nil
}

func (r *StubAuthRepository) UpdateBootstrapSecretDigest(ctx context.Context, secretDigest []byte) error {
	if r != nil && r.UpdateBootstrapFn != nil {
		return r.UpdateBootstrapFn(ctx, secretDigest)
	}
	return nil
}

func (r *StubAuthRepository) DeleteSessions(ctx context.Context, sessionIDs []string) error {
	if r != nil && r.DeleteSessionsFn != nil {
		return r.DeleteSessionsFn(ctx, sessionIDs)
	}
	return nil
}

func NewDeterministicAuthManagerWithRepository(t testing.TB, repo auth.Repository) *auth.Manager {
	t.Helper()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	sessionCounter := 0
	manager, err := auth.NewManager(
		auth.Config{
			SessionTTLDays: 1,
			SlidingRenewal: false,
			MaxSessions:    3,
		},
		auth.WithClock(func() time.Time {
			return current
		}),
		auth.WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		auth.WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-test-" + string(rune('0'+sessionCounter)), nil
		}),
		auth.WithRepository(repo),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

func NewDeterministicAuthManager(t testing.TB) *auth.Manager {
	t.Helper()

	manager, err := auth.NewManager(
		auth.Config{
			SessionTTLDays: 1,
			SlidingRenewal: false,
			MaxSessions:    3,
		},
		DeterministicAuthOptions()...,
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

func DeterministicAuthOptions() []auth.Option {
	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	sessionCounter := 0
	return []auth.Option{
		auth.WithClock(func() time.Time {
			return current
		}),
		auth.WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		auth.WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-test-" + string(rune('0'+sessionCounter)), nil
		}),
	}
}
