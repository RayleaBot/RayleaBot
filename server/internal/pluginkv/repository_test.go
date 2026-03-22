package pluginkv

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/storage"
)

func TestSQLiteRepositoryRoundTripAndList(t *testing.T) {
	t.Parallel()

	repo := openRepository(t)
	ctx := context.Background()
	limits := Limits{ValueMaxBytes: 1024, TotalMaxBytes: 4096}

	if err := repo.Set(ctx, "weather", "user:1:city", "上海", limits); err != nil {
		t.Fatalf("Set(city): %v", err)
	}
	if err := repo.Set(ctx, "weather", "user:1:units", map[string]any{"temp": "C"}, limits); err != nil {
		t.Fatalf("Set(units): %v", err)
	}

	value, exists, err := repo.Get(ctx, "weather", "user:1:city")
	if err != nil {
		t.Fatalf("Get(city): %v", err)
	}
	if !exists || value != "上海" {
		t.Fatalf("unexpected Get(city) result: exists=%v value=%#v", exists, value)
	}

	keys, err := repo.List(ctx, "weather", "user:1:")
	if err != nil {
		t.Fatalf("List(prefix): %v", err)
	}
	if len(keys) != 2 || keys[0] != "user:1:city" || keys[1] != "user:1:units" {
		t.Fatalf("unexpected keys: %#v", keys)
	}

	deleted, err := repo.Delete(ctx, "weather", "user:1:city")
	if err != nil {
		t.Fatalf("Delete(city): %v", err)
	}
	if !deleted {
		t.Fatal("Delete(city) = false, want true")
	}

	_, exists, err = repo.Get(ctx, "weather", "user:1:city")
	if err != nil {
		t.Fatalf("Get(city) after delete: %v", err)
	}
	if exists {
		t.Fatal("expected deleted key to be absent")
	}
}

func TestSQLiteRepositoryEnforcesValueAndTotalLimits(t *testing.T) {
	t.Parallel()

	repo := openRepository(t)
	ctx := context.Background()

	if err := repo.Set(ctx, "weather", "large", "123456", Limits{ValueMaxBytes: 5, TotalMaxBytes: 1024}); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("Set(value-too-large) error = %v, want ErrValueTooLarge", err)
	}

	limits := Limits{ValueMaxBytes: 64, TotalMaxBytes: 17}
	if err := repo.Set(ctx, "weather", "k1", "12345", limits); err != nil {
		t.Fatalf("Set(k1): %v", err)
	}
	if err := repo.Set(ctx, "weather", "k2", "67890", limits); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Set(quota) error = %v, want ErrQuotaExceeded", err)
	}
}

func TestSQLiteRepositoryDeleteMissingKeyReturnsFalse(t *testing.T) {
	t.Parallel()

	repo := openRepository(t)
	deleted, err := repo.Delete(context.Background(), "weather", "missing")
	if err != nil {
		t.Fatalf("Delete(missing): %v", err)
	}
	if deleted {
		t.Fatal("Delete(missing) = true, want false")
	}
}

func openRepository(t *testing.T) *SQLiteRepository {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	return repo
}
