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

func TestSQLiteRepositoryFiltersByBootIDAndTimeRange(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{LogID: "log_boot_0001", BootID: "boot_old", Timestamp: "2026-03-19T23:59:59Z", Level: "info", Source: "runtime", Message: "old boot"},
		{LogID: "log_boot_0002", BootID: "boot_new", Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "runtime", Message: "new boot first"},
		{LogID: "log_boot_0003", BootID: "boot_new", Timestamp: "2026-03-20T10:30:00Z", Level: "warn", Source: "runtime", Message: "new boot second"},
		{LogID: "log_boot_0004", BootID: "boot_new", Timestamp: "2026-03-21T00:00:00Z", Level: "error", Source: "runtime", Message: "out of range"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	currentSessionItems, err := repository.ListSummaries(ctx, Query{
		BootID: "boot_new",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("list summaries by boot id: %v", err)
	}
	if got := []string{currentSessionItems[0].Message, currentSessionItems[1].Message, currentSessionItems[2].Message}; !equalStrings(got, []string{"new boot first", "new boot second", "out of range"}) {
		t.Fatalf("unexpected boot-filtered summaries: %#v", got)
	}

	historyItems, err := repository.ListSummaries(ctx, Query{
		StartAt: "2026-03-20T00:00:00Z",
		EndAt:   "2026-03-20T23:59:59Z",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("list summaries by time range: %v", err)
	}
	if got := []string{historyItems[0].Message, historyItems[1].Message}; !equalStrings(got, []string{"new boot first", "new boot second"}) {
		t.Fatalf("unexpected range-filtered summaries: %#v", got)
	}
}

func TestSQLiteRepositoryFiltersTimeRangeAcrossTimezoneOffsets(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{LogID: "log_offset_0001", Timestamp: "2026-04-17T02:02:41+08:00", Level: "info", Source: "runtime", Message: "local offset row"},
		{LogID: "log_offset_0002", Timestamp: "2026-04-17T02:05:01+08:00", Level: "info", Source: "runtime", Message: "outside range"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	items, err := repository.ListSummaries(ctx, Query{
		StartAt: "2026-04-16T18:00:00Z",
		EndAt:   "2026-04-16T18:04:00Z",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("list summaries by UTC range: %v", err)
	}
	if got := []string{items[0].Message}; !equalStrings(got, []string{"local offset row"}) {
		t.Fatalf("unexpected offset-filtered summaries: %#v", got)
	}
}

func TestSQLiteRepositoryListsCursorPagedSummariesNewestFirst(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{LogID: "log_page_0001", Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "runtime", Message: "1"},
		{LogID: "log_page_0002", Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "runtime", Message: "2"},
		{LogID: "log_page_0003", Timestamp: "2026-03-20T10:00:01Z", Level: "info", Source: "runtime", Message: "3"},
		{LogID: "log_page_0004", Timestamp: "2026-03-20T10:00:02Z", Level: "info", Source: "runtime", Message: "4"},
		{LogID: "log_page_0005", Timestamp: "2026-03-20T10:00:03Z", Level: "info", Source: "runtime", Message: "5"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	firstPage, err := repository.ListPage(ctx, PageQuery{
		Source: "runtime",
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if got := []string{firstPage.Items[0].Message, firstPage.Items[1].Message}; !equalStrings(got, []string{"5", "4"}) {
		t.Fatalf("unexpected first page order: %#v", got)
	}
	if !firstPage.Page.HasOlder || firstPage.Page.HasNewer {
		t.Fatalf("unexpected first page metadata: %#v", firstPage.Page)
	}
	if firstPage.Page.OlderCursor == nil || firstPage.Page.NewerCursor != nil {
		t.Fatalf("unexpected first page cursors: %#v", firstPage.Page)
	}

	olderPage, err := repository.ListPage(ctx, PageQuery{
		Source:    "runtime",
		Limit:     2,
		Cursor:    *firstPage.Page.OlderCursor,
		Direction: PageDirectionOlder,
	})
	if err != nil {
		t.Fatalf("list older page: %v", err)
	}
	if got := []string{olderPage.Items[0].Message, olderPage.Items[1].Message}; !equalStrings(got, []string{"3", "2"}) {
		t.Fatalf("unexpected older page order: %#v", got)
	}
	if !olderPage.Page.HasOlder || !olderPage.Page.HasNewer {
		t.Fatalf("unexpected older page metadata: %#v", olderPage.Page)
	}
	if olderPage.Page.OlderCursor == nil || olderPage.Page.NewerCursor == nil {
		t.Fatalf("expected both cursors on middle page: %#v", olderPage.Page)
	}

	newerPage, err := repository.ListPage(ctx, PageQuery{
		Source:    "runtime",
		Limit:     2,
		Cursor:    *olderPage.Page.NewerCursor,
		Direction: PageDirectionNewer,
	})
	if err != nil {
		t.Fatalf("list newer page: %v", err)
	}
	if got := []string{newerPage.Items[0].Message, newerPage.Items[1].Message}; !equalStrings(got, []string{"5", "4"}) {
		t.Fatalf("unexpected newer page order: %#v", got)
	}
	if !newerPage.Page.HasOlder || newerPage.Page.HasNewer {
		t.Fatalf("unexpected newer page metadata: %#v", newerPage.Page)
	}
}

func TestSQLiteRepositoryPagesWithinBootIDScope(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{LogID: "log_boot_page_0001", BootID: "boot_old", Timestamp: "2026-03-20T09:59:59Z", Level: "info", Source: "runtime", Message: "old boot"},
		{LogID: "log_boot_page_0002", BootID: "boot_new", Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "runtime", Message: "1"},
		{LogID: "log_boot_page_0003", BootID: "boot_new", Timestamp: "2026-03-20T10:00:01Z", Level: "info", Source: "runtime", Message: "2"},
		{LogID: "log_boot_page_0004", BootID: "boot_new", Timestamp: "2026-03-20T10:00:02Z", Level: "info", Source: "runtime", Message: "3"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	firstPage, err := repository.ListPage(ctx, PageQuery{
		BootID: "boot_new",
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("list first boot-scoped page: %v", err)
	}
	if got := []string{firstPage.Items[0].Message, firstPage.Items[1].Message}; !equalStrings(got, []string{"3", "2"}) {
		t.Fatalf("unexpected first boot-scoped page: %#v", got)
	}
	if !firstPage.Page.HasOlder || firstPage.Page.HasNewer {
		t.Fatalf("unexpected first boot-scoped page info: %#v", firstPage.Page)
	}

	secondPage, err := repository.ListPage(ctx, PageQuery{
		BootID:    "boot_new",
		Limit:     2,
		Cursor:    *firstPage.Page.OlderCursor,
		Direction: PageDirectionOlder,
	})
	if err != nil {
		t.Fatalf("list second boot-scoped page: %v", err)
	}
	if got := []string{secondPage.Items[0].Message}; !equalStrings(got, []string{"1"}) {
		t.Fatalf("unexpected second boot-scoped page: %#v", got)
	}
	if secondPage.Page.HasOlder || !secondPage.Page.HasNewer {
		t.Fatalf("unexpected second boot-scoped page info: %#v", secondPage.Page)
	}
}

func TestSQLiteRepositoryRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	_, err := repository.ListPage(context.Background(), PageQuery{
		Limit:  2,
		Cursor: "not-a-valid-cursor",
	})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestSQLiteRepositoryIgnoresNewerDirectionWithoutCursorOnFirstPage(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	for _, summary := range []Summary{
		{LogID: "log_page_1001", Timestamp: "2026-03-20T10:00:00Z", Level: "info", Source: "runtime", Message: "1"},
		{LogID: "log_page_1002", Timestamp: "2026-03-20T10:00:01Z", Level: "info", Source: "runtime", Message: "2"},
		{LogID: "log_page_1003", Timestamp: "2026-03-20T10:00:02Z", Level: "info", Source: "runtime", Message: "3"},
		{LogID: "log_page_1004", Timestamp: "2026-03-20T10:00:03Z", Level: "info", Source: "runtime", Message: "4"},
		{LogID: "log_page_1005", Timestamp: "2026-03-20T10:00:04Z", Level: "info", Source: "runtime", Message: "5"},
	} {
		if err := repository.SaveSummary(ctx, summary); err != nil {
			t.Fatalf("save summary: %v", err)
		}
	}

	defaultPage, err := repository.ListPage(ctx, PageQuery{
		Source: "runtime",
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("list default page: %v", err)
	}

	newerFirstPage, err := repository.ListPage(ctx, PageQuery{
		Source:    "runtime",
		Limit:     2,
		Direction: PageDirectionNewer,
	})
	if err != nil {
		t.Fatalf("list first page with newer direction: %v", err)
	}

	if got := []string{newerFirstPage.Items[0].Message, newerFirstPage.Items[1].Message}; !equalStrings(got, []string{"5", "4"}) {
		t.Fatalf("unexpected first page order for direction=newer: %#v", got)
	}
	if newerFirstPage.Page.HasOlder != defaultPage.Page.HasOlder || newerFirstPage.Page.HasNewer != defaultPage.Page.HasNewer {
		t.Fatalf("unexpected first page metadata for direction=newer: %#v", newerFirstPage.Page)
	}
	if (newerFirstPage.Page.OlderCursor == nil) != (defaultPage.Page.OlderCursor == nil) || (newerFirstPage.Page.NewerCursor == nil) != (defaultPage.Page.NewerCursor == nil) {
		t.Fatalf("unexpected cursor presence for direction=newer: got %#v want %#v", newerFirstPage.Page, defaultPage.Page)
	}
	if newerFirstPage.Page.OlderCursor != nil && defaultPage.Page.OlderCursor != nil && *newerFirstPage.Page.OlderCursor != *defaultPage.Page.OlderCursor {
		t.Fatalf("unexpected older cursor for direction=newer: got %q want %q", *newerFirstPage.Page.OlderCursor, *defaultPage.Page.OlderCursor)
	}
	if newerFirstPage.Page.NewerCursor != nil && defaultPage.Page.NewerCursor != nil && *newerFirstPage.Page.NewerCursor != *defaultPage.Page.NewerCursor {
		t.Fatalf("unexpected newer cursor for direction=newer: got %q want %q", *newerFirstPage.Page.NewerCursor, *defaultPage.Page.NewerCursor)
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

func TestSQLiteRepositoryCompactsStoredOneBotDetailMirrorsOnRead(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	if _, err := repository.write.ExecContext(ctx, `INSERT INTO management_logs (log_id, ts, level, source, message, plugin_id, request_id, details_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"log_detail_0002",
		"2026-03-20T10:00:01Z",
		"info",
		"bridge",
		"runtime bridge delivered adapter event",
		"",
		"",
		`{
			"event_timestamp":1711015202,
			"time":1711015202,
			"conversation_id":"2001",
			"group_id":"2001",
			"message_id":"1001",
			"real_id":"1001",
			"message_seq":"1001",
			"sender_id":"3001",
			"sender_nickname":"Alice",
			"sender_role":"admin",
			"sender":{"user_id":"3001"}
		}`,
	); err != nil {
		t.Fatalf("insert raw detail summary: %v", err)
	}

	item, err := repository.GetSummary(ctx, "log_detail_0002")
	if err != nil {
		t.Fatalf("get detail summary: %v", err)
	}

	for _, key := range []string{"time", "group_id", "real_id", "message_seq", "sender_id", "sender_nickname", "sender_role"} {
		if _, ok := item.Details[key]; ok {
			t.Fatalf("detail key %q should be omitted: %#v", key, item.Details)
		}
	}

	sender, ok := item.Details["sender"].(map[string]any)
	if !ok {
		t.Fatalf("expected sender map, got %#v", item.Details["sender"])
	}
	if sender["user_id"] != "3001" || sender["nickname"] != "Alice" || sender["role"] != "admin" {
		t.Fatalf("unexpected sender details: %#v", sender)
	}
}

func TestSQLiteRepositorySanitizesStoredOneBotTextOnRead(t *testing.T) {
	t.Parallel()

	repository := openLoggingRepository(t)
	ctx := context.Background()

	if _, err := repository.write.ExecContext(ctx, `INSERT INTO management_logs (log_id, ts, level, source, message, plugin_id, request_id, details_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"log_detail_0003",
		"2026-03-20T10:00:02Z",
		"info",
		"bridge",
		"1145141919: [终末地摸鱼群(553855023)][管理员]群星怒/没错，是魔法！(1358252269): 除了战猎这种抓不到加费就完全没法打的角色",
		"",
		"",
		`{
			"plain_text":"hello\u202eworld",
			"sender":{"card":"群星怒\u2066~喵"}
		}`,
	); err != nil {
		t.Fatalf("insert raw detail summary: %v", err)
	}

	item, err := repository.GetSummary(ctx, "log_detail_0003")
	if err != nil {
		t.Fatalf("get detail summary: %v", err)
	}
	if item.Message != "1145141919: [终末地摸鱼群(553855023)][管理员]群星怒/没错，是魔法！(1358252269): 除了战猎这种抓不到加费就完全没法打的角色" {
		t.Fatalf("unexpected sanitized message: %#v", item.Message)
	}
	if got := item.Details["plain_text"]; got != "helloworld" {
		t.Fatalf("unexpected sanitized plain_text detail: %#v", got)
	}
	sender := item.Details["sender"].(map[string]any)
	if got := sender["card"]; got != "群星怒~喵" {
		t.Fatalf("unexpected sanitized sender card: %#v", got)
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

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
