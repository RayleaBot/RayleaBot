package permission

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSQLiteWhitelistRepositoryCRUDAndPreservesCreatedAt(t *testing.T) {
	t.Parallel()

	store := openPermissionTestStore(t)
	repo := NewSQLiteWhitelistRepository(store.Read, store.Write)
	ctx := context.Background()

	if err := repo.Add(ctx, "user", "10001", "值班账号"); err != nil {
		t.Fatalf("add whitelist entry: %v", err)
	}

	entry, err := repo.Get(ctx, "user", "10001")
	if err != nil {
		t.Fatalf("get whitelist entry: %v", err)
	}
	if entry.Reason != "值班账号" {
		t.Fatalf("reason = %q, want 值班账号", entry.Reason)
	}
	createdAt := entry.CreatedAt

	time.Sleep(20 * time.Millisecond)
	if err := repo.Add(ctx, "user", "10001", "轮值账号"); err != nil {
		t.Fatalf("upsert whitelist entry: %v", err)
	}

	updated, err := repo.Get(ctx, "user", "10001")
	if err != nil {
		t.Fatalf("get updated whitelist entry: %v", err)
	}
	if updated.Reason != "轮值账号" {
		t.Fatalf("reason = %q, want 轮值账号", updated.Reason)
	}
	if updated.CreatedAt != createdAt {
		t.Fatalf("created_at = %q, want %q", updated.CreatedAt, createdAt)
	}

	entries, err := repo.List(ctx, "user")
	if err != nil {
		t.Fatalf("list whitelist entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries length = %d, want 1", len(entries))
	}

	if err := repo.Remove(ctx, "user", "10001"); err != nil {
		t.Fatalf("remove whitelist entry: %v", err)
	}
	if _, err := repo.Get(ctx, "user", "10001"); err != ErrGovernanceEntryNotFound {
		t.Fatalf("get removed whitelist entry error = %v, want ErrGovernanceEntryNotFound", err)
	}
}

func TestSQLiteWhitelistStateRepository(t *testing.T) {
	t.Parallel()

	store := openPermissionTestStore(t)
	repo := NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	ctx := context.Background()

	enabled, err := repo.Enabled(ctx)
	if err != nil {
		t.Fatalf("read initial whitelist state: %v", err)
	}
	if enabled {
		t.Fatal("initial whitelist state should be disabled")
	}

	if err := repo.SetEnabled(ctx, true); err != nil {
		t.Fatalf("enable whitelist state: %v", err)
	}
	enabled, err = repo.Enabled(ctx)
	if err != nil {
		t.Fatalf("read enabled whitelist state: %v", err)
	}
	if !enabled {
		t.Fatal("whitelist state should be enabled")
	}

	if err := repo.SetEnabled(ctx, false); err != nil {
		t.Fatalf("disable whitelist state: %v", err)
	}
	enabled, err = repo.Enabled(ctx)
	if err != nil {
		t.Fatalf("read disabled whitelist state: %v", err)
	}
	if enabled {
		t.Fatal("whitelist state should be disabled")
	}
}

func openPermissionTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})
	return store
}
