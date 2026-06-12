package server

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
)

type stubAuthRepository struct {
	loadBootstrapFn   func(context.Context) (*auth.BootstrapState, error)
	loadSessionsFn    func(context.Context) ([]auth.Claims, error)
	saveBootstrapFn   func(context.Context, auth.BootstrapState, auth.Claims) error
	updateBootstrapFn func(context.Context, []byte) error
	saveSessionFn     func(context.Context, auth.Claims) error
	deleteSessionsFn  func(context.Context, []string) error
}

func (r *stubAuthRepository) LoadBootstrap(ctx context.Context) (*auth.BootstrapState, error) {
	if r != nil && r.loadBootstrapFn != nil {
		return r.loadBootstrapFn(ctx)
	}
	return nil, nil
}

func (r *stubAuthRepository) LoadSessions(ctx context.Context) ([]auth.Claims, error) {
	if r != nil && r.loadSessionsFn != nil {
		return r.loadSessionsFn(ctx)
	}
	return nil, nil
}

func (r *stubAuthRepository) SaveBootstrap(ctx context.Context, state auth.BootstrapState, claims auth.Claims) error {
	if r != nil && r.saveBootstrapFn != nil {
		return r.saveBootstrapFn(ctx, state, claims)
	}
	return nil
}

func (r *stubAuthRepository) SaveSession(ctx context.Context, claims auth.Claims) error {
	if r != nil && r.saveSessionFn != nil {
		return r.saveSessionFn(ctx, claims)
	}
	return nil
}

func (r *stubAuthRepository) UpdateBootstrapSecretDigest(ctx context.Context, secretDigest []byte) error {
	if r != nil && r.updateBootstrapFn != nil {
		return r.updateBootstrapFn(ctx, secretDigest)
	}
	return nil
}

func (r *stubAuthRepository) DeleteSessions(ctx context.Context, sessionIDs []string) error {
	if r != nil && r.deleteSessionsFn != nil {
		return r.deleteSessionsFn(ctx, sessionIDs)
	}
	return nil
}

func newDeterministicAuthManagerWithRepository(t *testing.T, repo auth.Repository) *auth.Manager {
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
