package secrets

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSQLiteStore_SetAndGet(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ss, err := NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	if err := ss.Set(ctx, "signing_key", []byte("test-key-bytes")); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := ss.Get(ctx, "signing_key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "test-key-bytes" {
		t.Errorf("got %q, want %q", got, "test-key-bytes")
	}
}

func TestSQLiteStore_GetNotFound(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ss, err := NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = ss.Get(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteStore_Upsert(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ss, err := NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	if err := ss.Set(ctx, "token", []byte("v1")); err != nil {
		t.Fatalf("set v1: %v", err)
	}
	if err := ss.Set(ctx, "token", []byte("v2")); err != nil {
		t.Fatalf("set v2: %v", err)
	}

	got, err := ss.Get(ctx, "token")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("got %q, want %q", got, "v2")
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ss, err := NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	if err := ss.Set(ctx, "ephemeral", []byte("data")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := ss.Delete(ctx, "ephemeral"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = ss.Get(ctx, "ephemeral")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLiteStore_List(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ss, err := NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	for _, key := range []string{"b_key", "a_key", "c_key"} {
		if err := ss.Set(ctx, key, []byte("val")); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	keys, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("got %d keys, want 3", len(keys))
	}
	// Should be sorted alphabetically.
	if keys[0] != "a_key" || keys[1] != "b_key" || keys[2] != "c_key" {
		t.Errorf("keys = %v, want [a_key b_key c_key]", keys)
	}
}

func TestSQLiteStore_NilStore(t *testing.T) {
	t.Parallel()
	_, err := NewSQLiteStore(nil)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}
