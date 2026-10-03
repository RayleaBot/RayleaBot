package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// This is the pre-index aggregate order used to compare historical SQLite
// values that cannot be produced by encodeValue's nonnegative integer sizes.
const legacyKVTotalSQL = `SELECT CAST(COALESCE(SUM(size_bytes),0) AS INTEGER) FROM plugin_kv NOT INDEXED WHERE expires_at_ms IS NULL OR expires_at_ms>?`

func sameSQLiteFailure(left, right error) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	var leftCode, rightCode interface{ Code() int }
	return errors.As(left, &leftCode) && errors.As(right, &rightCode) && leftCode.Code() == rightCode.Code()
}

func TestKVQuotaPreservesHistoricalAggregateResults(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		sizes   []any
		expired int
	}{
		{"signed overflow", []any{int64(9223372036854775807), int64(1), int64(-1)}, -1},
		{"fractional REAL", []any{0.1, 0.2, 0.7}, -1},
		{"large REAL", []any{1e19, -1e19, float64(1)}, -1},
		{"REAL after integer overflow", []any{int64(9223372036854775807), int64(1), 1.5}, -1},
		{"nonnegative overflow", []any{int64(9223372036854775807), int64(1)}, -1},
		{"expired negative", []any{int64(10), int64(-100), int64(2)}, 1},
		{"expired REAL", []any{int64(10), 1.5, int64(2)}, 1},
		{"TEXT", []any{int64(10), "not-a-size", int64(2)}, -1},
		{"BLOB", []any{int64(10), []byte("2"), int64(2)}, -1},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			repo, clock := timedRepository(t)
			for index, size := range fixture.sizes {
				var expiry any
				if index == fixture.expired {
					expiry = clock.Load()
				}
				if _, err := repo.write.Exec(`INSERT INTO plugin_kv(plugin_id,key,value_json,size_bytes,updated_at,expires_at_ms) VALUES(?,'fixture','1',?,'2026-10-03T00:00:00Z',?)`, fmt.Sprintf("plugin-%d", len(fixture.sizes)-index), size, expiry); err != nil {
					t.Fatal(err)
				}
			}
			var expected int64
			expectedErr := repo.read.QueryRow(legacyKVTotalSQL, clock.Load()).Scan(&expected)
			actual, actualErr := repo.readQ.GetKVTotalSize(t.Context(), sql.NullInt64{Int64: clock.Load(), Valid: true})
			if !sameSQLiteFailure(actualErr, expectedErr) || actualErr == nil && actual != expected {
				t.Fatalf("historical aggregate changed: got=%d/%v want=%d/%v", actual, actualErr, expected, expectedErr)
			}
			if expectedErr != nil {
				err := repo.Set(t.Context(), "new-plugin", "new", 1, KVLimits{})
				if !sameSQLiteFailure(err, expectedErr) {
					t.Fatalf("write no longer preserves aggregate failure: %v", err)
				}
				var count int
				if err := repo.read.QueryRow(`SELECT COUNT(*) FROM plugin_kv WHERE plugin_id='new-plugin'`).Scan(&count); err != nil || count != 0 {
					t.Fatal("failed quota accounting persisted a write")
				}
			}
		})
	}
}

func TestKVQuotaIndexTracksLegalWritesAndLegacyReplacement(t *testing.T) {
	repo, clock := timedRepository(t)
	var expected int64
	for index, value := range []any{nil, true, "状态", map[string]any{"cursor": float64(3)}, []any{1, "two"}} {
		key := fmt.Sprintf("key-%d", index)
		if err := repo.Set(t.Context(), "fixture", key, value, KVLimits{}); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		expected += int64(len(key) + len(encoded))
	}
	actual, err := repo.readQ.GetKVTotalSize(t.Context(), sql.NullInt64{Int64: clock.Load(), Valid: true})
	if err != nil || actual != expected {
		t.Fatalf("legal encoded byte sizes changed: %d != %d, %v", actual, expected, err)
	}
	var anomalies int
	if err := repo.read.QueryRow(`SELECT COUNT(*) FROM plugin_kv INDEXED BY idx_plugin_kv_size_anomaly WHERE typeof(size_bytes)<>'integer' OR size_bytes<0`).Scan(&anomalies); err != nil || anomalies != 0 {
		t.Fatalf("legal writes entered the anomaly index: %d %v", anomalies, err)
	}
	if _, err := repo.write.Exec(`INSERT INTO plugin_kv(plugin_id,key,value_json,size_bytes,updated_at,expires_at_ms) VALUES('legacy','key','1',-1,'2026-10-03T00:00:00Z',?)`, clock.Load()+1); err != nil {
		t.Fatal(err)
	}
	for _, now := range []int64{clock.Load(), clock.Load() + 1} {
		var want int64
		if err := repo.read.QueryRow(legacyKVTotalSQL, now).Scan(&want); err != nil {
			t.Fatal(err)
		}
		got, err := repo.readQ.GetKVTotalSize(t.Context(), sql.NullInt64{Int64: now, Valid: true})
		if err != nil || got != want {
			t.Fatalf("legacy anomaly expiration boundary changed: %d != %d %v", got, want, err)
		}
	}
	if err := repo.Set(t.Context(), "legacy", "key", 2, KVLimits{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.read.QueryRow(`SELECT COUNT(*) FROM plugin_kv INDEXED BY idx_plugin_kv_size_anomaly WHERE typeof(size_bytes)<>'integer' OR size_bytes<0`).Scan(&anomalies); err != nil || anomalies != 0 {
		t.Fatalf("valid replacement retained its historical anomaly: %d %v", anomalies, err)
	}
}

func TestKVConcurrentPluginsShareLiveQuota(t *testing.T) {
	repo, clock := timedRepository(t)
	const writers = 8
	// key "k" plus encoded value "1" costs two bytes; exactly four fit.
	limits := KVLimits{ValueMaxBytes: 64, TotalMaxBytes: 8}
	for round := range 2 {
		start := make(chan struct{})
		results := make(chan error, writers)
		var workers sync.WaitGroup
		for writer := range writers {
			workers.Go(func() {
				<-start
				_, err := repo.SetWithOptions(context.Background(), fmt.Sprintf("plugin-%d-%d", round, writer), "k", 1, limits, KVSetOptions{TTLSeconds: 1})
				results <- err
			})
		}
		close(start)
		workers.Wait()
		close(results)
		accepted, rejected := 0, 0
		for err := range results {
			if err == nil {
				accepted++
			} else if errors.Is(err, ErrKVQuotaExceeded) {
				rejected++
			} else {
				t.Fatal(err)
			}
		}
		if accepted != 4 || rejected != 4 {
			t.Fatalf("concurrent global quota: accepted=%d rejected=%d", accepted, rejected)
		}
		total, err := repo.readQ.GetKVTotalSize(t.Context(), sql.NullInt64{Int64: clock.Load(), Valid: true})
		if err != nil || total != 8 {
			t.Fatalf("live global size = %d, %v", total, err)
		}
		clock.Add(1000)
	}
}

func TestKVOverwriteNormalizesHistoricalMetadata(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		size   any
		expiry any
		ttl    int
	}{
		{"BLOB size", []byte("2"), nil, 0},
		{"TEXT expiry", int64(2), "historical-deadline", 0},
		{"REAL expiry", int64(2), float64(1000000.5), 1},
		{"BLOB expiry", int64(2), []byte("1001000"), 1},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			repo, clock := timedRepository(t)
			if _, err := repo.write.Exec(`INSERT INTO plugin_kv(plugin_id,key,value_json,size_bytes,updated_at,expires_at_ms) VALUES('fixture','k','0',?,'2026-10-03T00:00:00Z',?)`, fixture.size, fixture.expiry); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.SetWithOptions(t.Context(), "fixture", "k", 1, KVLimits{}, KVSetOptions{TTLSeconds: fixture.ttl}); err != nil {
				t.Fatalf("historical metadata overwrite newly failed: %v", err)
			}
			row := readKVStoredRow(t, repo, "fixture", "k")
			wantExpiryType := "null"
			if fixture.ttl > 0 {
				wantExpiryType = "integer"
			}
			if row.ValueJSON != "1" || row.Size != 2 || row.SizeType != "integer" || row.ExpiryType != wantExpiryType || fixture.ttl > 0 && (!row.Expiry.Valid || row.Expiry.Int64 != clock.Load()+1000) {
				t.Fatalf("overwrite did not normalize metadata types: %+v", row)
			}
		})
	}
}
