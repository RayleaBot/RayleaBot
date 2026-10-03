package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
)

func TestLogWriteRetriesAfterPrepareFailure(t *testing.T) {
	t.Parallel()
	repository, store := openLoggingRepositoryStore(t)
	summary := logging.Summary{LogID: "fixture-retry", Timestamp: "2026-10-03T00:00:00Z", Level: "info", Source: "fixture", Message: "retry"}
	if _, err := store.Write.Exec(`ALTER TABLE management_logs RENAME TO unavailable_logs`); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveSummary(t.Context(), summary); err == nil {
		t.Fatal("write succeeded without its table")
	}
	if _, err := store.Write.Exec(`ALTER TABLE unavailable_logs RENAME TO management_logs`); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := repository.SaveSummary(canceled, summary); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled first write = %v", err)
	}
	if err := repository.SaveSummary(t.Context(), summary); err != nil {
		t.Fatal(err)
	}
	if got, err := repository.GetSummary(t.Context(), summary.LogID); err != nil || got.Message != "retry" {
		t.Fatalf("recovered write = %+v, %v", got, err)
	}
}

func TestPreparingLogCleanupDoesNotDelayCanceledWrites(t *testing.T) {
	t.Parallel()
	repository, store := openLoggingRepositoryStore(t)
	summary := logging.Summary{LogID: "fixture-original", Timestamp: "2026-10-03T00:00:00Z", Level: "info", Source: "fixture", Message: "fixture"}
	if err := repository.SaveSummary(t.Context(), summary); err != nil {
		t.Fatal(err)
	}
	connection, err := store.Write.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	preparing, cancelPrepare := context.WithCancel(t.Context())
	defer cancelPrepare()
	waitCount := store.Write.Stats().WaitCount
	pruned := make(chan error, 1)
	go func() { pruned <- repository.PruneOlderThan(preparing, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)) }()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for store.Write.Stats().WaitCount == waitCount {
		select {
		case <-poll.C:
		case <-deadline.C:
			t.Fatal("cleanup did not wait for the write connection")
		}
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	summary.LogID = "fixture-after-cancel"
	saved := make(chan error, 1)
	go func() { saved <- repository.SaveSummary(canceled, summary) }()
	select {
	case err := <-saved:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled write = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("statement preparation delayed another caller's cancellation")
	}
	cancelPrepare()
	if err := <-pruned; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled cleanup = %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveSummary(t.Context(), summary); err != nil {
		t.Fatal(err)
	}
	if err := repository.PruneOlderThan(t.Context(), time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetSummary(t.Context(), summary.LogID); !errors.Is(err, logging.ErrLogNotFound) {
		t.Fatalf("cleanup retry did not remove expired log: %v", err)
	}
}

func TestConcurrentLogWritesKeepSeparateBindings(t *testing.T) {
	t.Parallel()
	repository := openLoggingRepository(t)
	start := make(chan struct{})
	var writers sync.WaitGroup
	for index := range 8 {
		writers.Go(func() {
			<-start
			id := fmt.Sprintf("fixture-%d", index)
			if err := repository.SaveSummary(t.Context(), logging.Summary{LogID: id, Timestamp: "2026-10-03T00:00:00Z", Level: "info", Source: "fixture", Message: id}); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	writers.Wait()
	for index := range 8 {
		id := fmt.Sprintf("fixture-%d", index)
		if got, err := repository.GetSummary(t.Context(), id); err != nil || got.Message != id {
			t.Fatalf("concurrent write %s = %+v, %v", id, got, err)
		}
	}
}
