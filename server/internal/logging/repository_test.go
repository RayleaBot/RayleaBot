package logging

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSQLiteRepositoryListsFilteredSummariesInAscendingOrder(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{Timestamp: "2026-03-20T10:00:02Z", Level: "error", Source: "runtime", Message: "third", PluginID: "weather"},
		{Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "server", Message: "first"},
		{Timestamp: "2026-03-20T10:00:01Z", Level: "error", Source: "runtime", Message: "second", PluginID: "weather", RequestID: "req_1"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	items, err := repository.ListSummaries(ctx, Query{
		Level:    "error",
		PluginID: "weather",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected summary count: got %d want 2", len(items))
	}
	if items[0].Message != "second" || items[1].Message != "third" {
		t.Fatalf("unexpected summary order: %#v", items)
	}
}

func TestSQLiteRepositoryPrunesOldSummaries(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	oldSummary := Summary{
		Timestamp: "2026-03-10T10:00:00Z",
		Level:     "warn",
		Source:    "runtime",
		Message:   "old",
	}
	newSummary := Summary{
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "info",
		Source:    "server",
		Message:   "new",
	}
	if err := repository.SaveSummary(ctx, oldSummary); err != nil {
		t.Fatalf("save old summary: %v", err)
	}
	if err := repository.SaveSummary(ctx, newSummary); err != nil {
		t.Fatalf("save new summary: %v", err)
	}

	if err := repository.PruneOlderThan(ctx, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("prune summaries: %v", err)
	}

	items, err := repository.ListSummaries(ctx, Query{Limit: 10})
	if err != nil {
		t.Fatalf("list summaries after prune: %v", err)
	}
	if len(items) != 1 || items[0].Message != "new" {
		t.Fatalf("unexpected summaries after prune: %#v", items)
	}
}

func openLoggingRepository(t *testing.T) *SQLiteRepository {
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

	repository, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("create sqlite logging repository: %v", err)
	}
	return repository
}
