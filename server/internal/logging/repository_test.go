package logging

import (
	"context"
	"errors"
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
	for _, item := range items {
		if item.LogID == "" {
			t.Fatalf("expected log_id to be populated: %#v", item)
		}
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

func TestSQLiteRepositoryFiltersByDerivedProtocol(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{Timestamp: "2026-03-20T10:00:00Z", Level: "warn", Source: "adapter", Message: "adapter"},
		{Timestamp: "2026-03-20T10:00:01Z", Level: "warn", Source: "adapter.onebot11", Message: "adapter.onebot11"},
		{Timestamp: "2026-03-20T10:00:02Z", Level: "info", Source: "bridge", Message: "bridge"},
		{Timestamp: "2026-03-20T10:00:03Z", Level: "info", Source: "runtime", Message: "runtime"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	items, err := repository.ListSummaries(ctx, Query{
		Protocol: ProtocolOneBot11,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list summaries by protocol: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("unexpected protocol summary count: got %d want 3", len(items))
	}
	for _, item := range items {
		if item.Protocol != ProtocolOneBot11 {
			t.Fatalf("unexpected summary protocol: %#v", item)
		}
	}
}

func TestSQLiteRepositoryGetsDetailAndSanitizesSensitiveKeys(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	if err := repository.SaveSummary(ctx, Summary{
		LogID:     "log_detail_0001",
		Timestamp: "2026-03-20T10:00:00Z",
		Level:     "warn",
		Source:    "adapter.onebot11",
		Message:   "ignored OneBot API response with unsupported echo",
		RequestID: "req_adapter_ignored",
		Details: map[string]any{
			"direction":       "inbound",
			"echo_value_type": "float64",
			"payload_preview": map[string]any{"status": "ok"},
			"token":           "should-not-survive",
		},
	}); err != nil {
		t.Fatalf("save detail summary: %v", err)
	}

	item, err := repository.GetSummary(ctx, "log_detail_0001")
	if err != nil {
		t.Fatalf("get detail summary: %v", err)
	}
	if item.LogID != "log_detail_0001" {
		t.Fatalf("unexpected log_id: %#v", item.LogID)
	}
	if item.Protocol != ProtocolOneBot11 {
		t.Fatalf("unexpected protocol: %#v", item.Protocol)
	}
	if item.Details["echo_value_type"] != "float64" {
		t.Fatalf("unexpected details: %#v", item.Details)
	}
	if _, ok := item.Details["token"]; ok {
		t.Fatalf("sensitive detail key should be removed: %#v", item.Details)
	}
}

func TestSQLiteRepositoryReturnsNotFoundForMissingLogID(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	_, err := repository.GetSummary(context.Background(), "log_missing_0001")
	if !errors.Is(err, ErrLogNotFound) {
		t.Fatalf("expected ErrLogNotFound, got %v", err)
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
