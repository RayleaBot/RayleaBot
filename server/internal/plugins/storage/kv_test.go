package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSQLiteRepositoryRoundTripAndList(t *testing.T) {
	t.Parallel()

	repo := openRepository(t)
	ctx := context.Background()
	limits := KVLimits{ValueMaxBytes: 1024, TotalMaxBytes: 4096}

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

	if err := repo.Set(ctx, "weather", "large", "123456", KVLimits{ValueMaxBytes: 5, TotalMaxBytes: 1024}); !errors.Is(err, ErrKVValueTooLarge) {
		t.Fatalf("Set(value-too-large) error = %v, want ErrKVValueTooLarge", err)
	}

	limits := KVLimits{ValueMaxBytes: 64, TotalMaxBytes: 17}
	if err := repo.Set(ctx, "weather", "k1", "12345", limits); err != nil {
		t.Fatalf("Set(k1): %v", err)
	}
	if err := repo.Set(ctx, "weather", "k2", "67890", limits); !errors.Is(err, ErrKVQuotaExceeded) {
		t.Fatalf("Set(quota) error = %v, want ErrKVQuotaExceeded", err)
	}
}

func TestSQLiteRepositoryEnforcesTotalLimitAcrossPlugins(t *testing.T) {
	t.Parallel()

	repo := openRepository(t)
	ctx := context.Background()
	limits := KVLimits{ValueMaxBytes: 64, TotalMaxBytes: 17}
	if err := repo.Set(ctx, "weather", "k1", "12345", limits); err != nil {
		t.Fatalf("Set(weather): %v", err)
	}
	if err := repo.Set(ctx, "subscription", "k2", "67890", limits); !errors.Is(err, ErrKVQuotaExceeded) {
		t.Fatalf("Set(global quota) error = %v, want ErrKVQuotaExceeded", err)
	}
	if err := repo.Set(ctx, "subscription", "k2", "67890", KVLimits{ValueMaxBytes: 64, TotalMaxBytes: 32}); err != nil {
		t.Fatal(err)
	}
	// Lowering the cap still rejects an equal-sized overwrite above that cap.
	if err := repo.Set(ctx, "weather", "k1", "abcde", limits); !errors.Is(err, ErrKVQuotaExceeded) {
		t.Fatalf("equal-sized overwrite bypassed global quota: %v", err)
	}
	if value, exists, err := repo.Get(ctx, "weather", "k1"); err != nil || !exists || value != "12345" {
		t.Fatalf("quota rejection changed the existing value: %v %v %v", value, exists, err)
	}
}

type kvStoredRow struct {
	ValueJSON, UpdatedAt, SizeType, ExpiryType string
	Size                                       int64
	Expiry                                     sql.NullInt64
}

func readKVStoredRow(t *testing.T, repo *KVSQLiteRepository, pluginID, key string) kvStoredRow {
	t.Helper()
	var row kvStoredRow
	if err := repo.read.QueryRow(`SELECT value_json,size_bytes,updated_at,expires_at_ms,typeof(size_bytes),typeof(expires_at_ms) FROM plugin_kv WHERE plugin_id=? AND key=?`, pluginID, key).Scan(&row.ValueJSON, &row.Size, &row.UpdatedAt, &row.Expiry, &row.SizeType, &row.ExpiryType); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSQLiteRepositoryOverwritesValueSizeAndTimestamp(t *testing.T) {
	repo, clock := timedRepository(t)
	for _, value := range []string{"first", "other", "other", "longer value"} {
		clock.Add(1)
		if err := repo.Set(t.Context(), "fixture", "key", value, KVLimits{}); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		row := readKVStoredRow(t, repo, "fixture", "key")
		if row.ValueJSON != string(encoded) || row.Size != int64(len("key")+len(encoded)) || row.UpdatedAt != time.UnixMilli(clock.Load()).UTC().Format(time.RFC3339Nano) || row.Expiry.Valid {
			t.Fatalf("overwrite lost value, byte size, timestamp or permanent lifetime: %+v", row)
		}
	}
}

func TestSQLiteRepositoryOverwritePropagatesDatabaseFailure(t *testing.T) {
	for _, value := range []string{"new", "longer"} {
		t.Run(value, func(t *testing.T) {
			repo, clock := timedRepository(t)
			if err := repo.Set(t.Context(), "fixture", "key", "old", KVLimits{}); err != nil {
				t.Fatal(err)
			}
			before := readKVStoredRow(t, repo, "fixture", "key")
			if _, err := repo.write.Exec(`CREATE TRIGGER reject_kv_update BEFORE UPDATE ON plugin_kv BEGIN SELECT RAISE(ABORT,'fixture write rejected'); END`); err != nil {
				t.Fatal(err)
			}
			clock.Add(1)
			err := repo.Set(t.Context(), "fixture", "key", value, KVLimits{})
			var databaseError interface{ Code() int }
			if !errors.As(err, &databaseError) || readKVStoredRow(t, repo, "fixture", "key") != before {
				t.Fatalf("database failure was lost or partially applied: %v", err)
			}
		})
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

func openRepository(t *testing.T) *KVSQLiteRepository {
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

	repo, err := NewKVSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewKVSQLiteRepository: %v", err)
	}
	return repo
}
