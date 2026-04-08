package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestRepositoryBackedManagerReloadsBootstrapAndSessions(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	databasePath := filepath.Join(t.TempDir(), "state.db")

	managerA, closeA := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    3,
	}, func() time.Time {
		return current
	}, "stage2-a")
	bootstrapToken, _, err := managerA.Bootstrap("admin", "fixture-only-secret")
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
	loginToken, _, err := managerA.Login("admin", "fixture-only-secret")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if err := closeA(); err != nil {
		t.Fatalf("close persistent manager A: %v", err)
	}

	managerB, closeB := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    3,
	}, func() time.Time {
		return current
	}, "stage2-b")
	defer func() {
		if err := closeB(); err != nil {
			t.Fatalf("close persistent manager B: %v", err)
		}
	}()

	if !managerB.IsBootstrapped() {
		t.Fatalf("expected bootstrap state to survive restart")
	}
	if _, err := managerB.Validate(bootstrapToken); err != nil {
		t.Fatalf("Validate bootstrap token after restart failed: %v", err)
	}
	if _, err := managerB.Validate(loginToken); err != nil {
		t.Fatalf("Validate login token after restart failed: %v", err)
	}
	if _, _, err := managerB.Login("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("Login after restart failed: %v", err)
	}
}

func TestRepositoryBackedManagerPrunesExpiredSessionsAcrossRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	databasePath := filepath.Join(t.TempDir(), "state.db")

	managerA, closeA := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    1,
	}, func() time.Time {
		return current
	}, "prune-a")
	if _, _, err := managerA.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
	if err := closeA(); err != nil {
		t.Fatalf("close persistent manager A: %v", err)
	}

	current = current.Add(24*time.Hour + time.Second)

	managerB, closeB := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    1,
	}, func() time.Time {
		return current
	}, "prune-b")
	defer func() {
		if err := closeB(); err != nil {
			t.Fatalf("close persistent manager B: %v", err)
		}
	}()

	if _, _, err := managerB.Login("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("expected expired persisted session to be pruned, got %v", err)
	}
}

func TestRepositoryBackedManagerPersistsSlidingRenewal(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	databasePath := filepath.Join(t.TempDir(), "state.db")

	managerA, closeA := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: true,
		MaxSessions:    2,
	}, func() time.Time {
		return current
	}, "renew-a")
	token, _, err := managerA.Bootstrap("admin", "fixture-only-secret")
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	current = current.Add(12 * time.Hour)
	if _, err := managerA.Validate(token); err != nil {
		t.Fatalf("Validate with sliding renewal failed: %v", err)
	}
	if err := closeA(); err != nil {
		t.Fatalf("close persistent manager A: %v", err)
	}

	current = time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)

	managerB, closeB := newPersistentManager(t, databasePath, Config{
		SessionTTLDays: 1,
		SlidingRenewal: true,
		MaxSessions:    2,
	}, func() time.Time {
		return current
	}, "renew-b")
	defer func() {
		if err := closeB(); err != nil {
			t.Fatalf("close persistent manager B: %v", err)
		}
	}()

	if _, err := managerB.Validate(token); err != nil {
		t.Fatalf("expected renewed token to survive restart, got %v", err)
	}
}

func newPersistentManager(t *testing.T, databasePath string, cfg Config, now func() time.Time, sessionPrefix string) (*Manager, func() error) {
	t.Helper()

	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	repository, err := NewSQLiteRepository(store)
	if err != nil {
		_ = store.Close()
		t.Fatalf("NewSQLiteRepository failed: %v", err)
	}

	sessionCounter := 0
	manager, err := NewManager(
		cfg,
		WithClock(now),
		WithRepository(repository),
		WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return sessionPrefix + "-" + string(rune('0'+sessionCounter)), nil
		}),
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager, store.Close
}
