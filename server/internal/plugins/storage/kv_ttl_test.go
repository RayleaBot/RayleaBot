package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corestore "github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func timedRepository(t *testing.T) (*KVSQLiteRepository, *atomic.Int64) {
	t.Helper()
	repo := openRepository(t)
	clock := &atomic.Int64{}
	clock.Store(1000000)
	repo.now = func() time.Time { return time.UnixMilli(clock.Load()) }
	return repo, clock
}

func TestKVExpiryBoundaryOverwriteAndPrefix(t *testing.T) {
	t.Parallel()
	repo, clock := timedRepository(t)
	ctx := t.Context()
	result, err := repo.SetWithOptions(ctx, "p", "%_\\:one", nil, KVLimits{}, KVSetOptions{TTLSeconds: 1})
	if err != nil || !result.Stored || result.ExpiresAtMS == nil || *result.ExpiresAtMS != 1001000 {
		t.Fatalf("set=%#v err=%v", result, err)
	}
	clock.Store(1000999)
	entry, err := repo.GetEntry(ctx, "p", "%_\\:one")
	if err != nil || !entry.Exists || entry.Value != nil || entry.ExpiresAtMS == nil {
		t.Fatalf("live entry=%#v err=%v", entry, err)
	}
	keys, err := repo.List(ctx, "p", "%_\\:")
	if err != nil || len(keys) != 1 {
		t.Fatalf("escaped prefix=%v err=%v", keys, err)
	}
	clock.Store(1001000)
	entry, err = repo.GetEntry(ctx, "p", "%_\\:one")
	if err != nil || entry.Exists || entry.ExpiresAtMS != nil || entry.Value != nil {
		t.Fatalf("expired entry leaked: %#v %v", entry, err)
	}
	keys, err = repo.List(ctx, "p", "%_\\:")
	if err != nil || keys == nil || len(keys) != 0 {
		t.Fatalf("expired list=%v err=%v", keys, err)
	}
	if deleted, err := repo.Delete(ctx, "p", "%_\\:one"); err != nil || deleted {
		t.Fatal("expired delete reported a live deletion")
	}
	if _, err := repo.SetWithOptions(ctx, "p", "overwrite", 1, KVLimits{}, KVSetOptions{TTLSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "p", "overwrite", 2, KVLimits{}); err != nil {
		t.Fatal(err)
	}
	clock.Store(2000000)
	entry, err = repo.GetEntry(ctx, "p", "overwrite")
	if err != nil || !entry.Exists || entry.Value != float64(2) || entry.ExpiresAtMS != nil {
		t.Fatal("permanent overwrite retained TTL")
	}
}

func TestKVNXConcurrentUniqueAndGlobalQuota(t *testing.T) {
	t.Parallel()
	repo, clock := timedRepository(t)
	ctx := t.Context()
	var contenders sync.WaitGroup
	var wins atomic.Int32
	failures := make(chan error, 64)
	for i := range 64 {
		contenders.Go(func() {
			result, err := repo.SetWithOptions(ctx, "p", "once", i, KVLimits{}, KVSetOptions{TTLSeconds: 1, IfNotExists: true})
			if err != nil {
				failures <- err
				return
			}
			if result.Stored {
				wins.Add(1)
			}
			if result.ExpiresAtMS == nil || *result.ExpiresAtMS != 1001000 {
				failures <- fmt.Errorf("unexpected NX expiry: %#v", result)
			}
		})
	}
	contenders.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if wins.Load() != 1 {
		t.Fatalf("NX winners=%d", wins.Load())
	}
	before, _ := repo.GetEntry(ctx, "p", "once")
	result, err := repo.SetWithOptions(ctx, "p", "once", "replacement", KVLimits{ValueMaxBytes: 64, TotalMaxBytes: 1}, KVSetOptions{TTLSeconds: 60, IfNotExists: true})
	if err != nil || result.Stored || result.ExpiresAtMS == nil || *result.ExpiresAtMS != 1001000 {
		t.Fatalf("NX no-op charged quota: %#v %v", result, err)
	}
	after, _ := repo.GetEntry(ctx, "p", "once")
	if before.Value != after.Value {
		t.Fatal("NX loser overwrote winner")
	}
	if _, err := repo.SetWithOptions(ctx, "p", "once", "too-large", KVLimits{ValueMaxBytes: 1}, KVSetOptions{IfNotExists: true}); !errors.Is(err, ErrKVValueTooLarge) {
		t.Fatal("NX skipped single-value validation")
	}
	if _, err := repo.SetWithOptions(ctx, "other", "k", 0, KVLimits{TotalMaxBytes: 3}, KVSetOptions{}); !errors.Is(err, ErrKVQuotaExceeded) {
		t.Fatal("quota became per-plugin")
	}
	clock.Store(1001000)
	if result, err := repo.SetWithOptions(ctx, "other", "k", 0, KVLimits{TotalMaxBytes: 3}, KVSetOptions{IfNotExists: true}); err != nil || !result.Stored {
		t.Fatalf("expired row retained global quota: %#v %v", result, err)
	}
	if result, err := repo.SetWithOptions(ctx, "p", "once", 5, KVLimits{}, KVSetOptions{IfNotExists: true}); err != nil || !result.Stored || result.ExpiresAtMS != nil {
		t.Fatal("expired NX did not become permanent")
	}
}

func TestKVWriteCapturesTimeOnceInsideTransaction(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	var calls atomic.Int64
	repo.now = func() time.Time { return time.UnixMilli(calls.Add(1) * 1000000) }
	result, err := repo.SetWithOptions(t.Context(), "p", "k", 1, KVLimits{}, KVSetOptions{TTLSeconds: 1})
	if err != nil || calls.Load() != 1 || result.ExpiresAtMS == nil || *result.ExpiresAtMS != 1001000 {
		t.Fatalf("time captures=%d result=%#v err=%v", calls.Load(), result, err)
	}
}

func TestKVExpirySurvivesReopenAndSnapshotRestore(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := corestore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, _ := NewKVSQLiteRepository(store)
	repo.now = func() time.Time { return time.UnixMilli(1000000) }
	if _, err := repo.SetWithOptions(t.Context(), "p", "k", 1, KVLimits{}, KVSetOptions{TTLSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.CreateSnapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, reopen := range []string{path, snapshot} {
		instance, err := corestore.Open(reopen)
		if err != nil {
			t.Fatal(err)
		}
		restored, _ := NewKVSQLiteRepository(instance)
		restored.now = func() time.Time { return time.UnixMilli(1001000) }
		entry, err := restored.GetEntry(t.Context(), "p", "k")
		if err != nil || entry.Exists {
			t.Fatal("expired value revived after restart/restore")
		}
		if err := instance.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestKVSweeperUsesExpiryIndexAndBoundedBatches(t *testing.T) {
	t.Parallel()
	repo, _ := timedRepository(t)
	if _, err := repo.write.Exec(`WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<6200)
INSERT INTO plugin_kv(plugin_id,key,value_json,size_bytes,updated_at,expires_at_ms)
SELECT 'p',printf('k%05d',i),'1',8,'2026-09-13T00:00:00Z',1000000 FROM n`); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.read.Query("EXPLAIN QUERY PLAN SELECT rowid FROM plugin_kv WHERE expires_at_ms IS NOT NULL AND expires_at_ms <= ? ORDER BY expires_at_ms,rowid LIMIT 1000", 1000000)
	if err != nil {
		t.Fatal(err)
	}
	indexed := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		indexed = indexed || strings.Contains(detail, "idx_plugin_kv_expiry")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("sweep would scan all KV rows")
	}
	start := time.Now()
	count, err := repo.SweepExpired(t.Context())
	if err != nil || count <= 0 || count > 5000 || time.Since(start) > time.Second {
		t.Fatalf("unbounded sweep: rows=%d time=%v err=%v", count, time.Since(start), err)
	}
	var remaining int64
	if err := repo.read.QueryRow("SELECT COUNT(*) FROM plugin_kv").Scan(&remaining); err != nil || remaining != 6200-count {
		t.Fatal("sweep count did not reflect actual committed rows")
	}
	if err := repo.Set(t.Context(), "q", "normal", true, KVLimits{}); err != nil {
		t.Fatal("sweep did not release writer")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := repo.SweepExpired(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("sweep ignored owner cancellation")
	}
}
