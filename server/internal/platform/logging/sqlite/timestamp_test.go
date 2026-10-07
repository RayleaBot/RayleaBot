package sqlite

import (
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
)

func TestTimestampsKeepExactOrderAndCursorBoundaries(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, row := range []struct{ id, timestamp string }{
		{"current", "2026-09-22T00:00:00.000000002Z"},
		{"whole", "2026-09-22T00:00:00Z"},
		{"variable", "2026-09-22T00:00:00.1Z"},
		{"offset", "2026-09-22T08:00:00.000000001+08:00"},
	} {
		// 写入不同小数精度与时区形式，验证它们对应的绝对时刻。
		if err := repository.SaveSummary(t.Context(), logging.Summary{
			LogID: row.id, Timestamp: row.timestamp, Level: "info", Source: "fixture", Message: row.id,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(items []logging.Summary) []string {
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.LogID)
		}
		return result
	}
	items, err := repository.ListSummaries(t.Context(), logging.Query{Limit: 10})
	if err != nil || !equalStrings(ids(items), []string{"whole", "offset", "current", "variable"}) {
		t.Fatalf("replay order = %v, %v", ids(items), err)
	}
	first, err := repository.ListPage(t.Context(), logging.PageQuery{Limit: 2})
	if err != nil || !equalStrings(ids(first.Items), []string{"variable", "current"}) || first.Page.OlderCursor == nil {
		t.Fatalf("first page = %#v, %v", first, err)
	}
	older, err := repository.ListPage(t.Context(), logging.PageQuery{Limit: 2, Cursor: *first.Page.OlderCursor, Direction: logging.PageDirectionOlder})
	if err != nil || !equalStrings(ids(older.Items), []string{"offset", "whole"}) || older.Page.NewerCursor == nil || older.Page.HasOlder {
		t.Fatalf("older page = %#v, %v", older, err)
	}
	newer, err := repository.ListPage(t.Context(), logging.PageQuery{Limit: 2, Cursor: *older.Page.NewerCursor, Direction: logging.PageDirectionNewer})
	if err != nil || !equalStrings(ids(newer.Items), []string{"variable", "current"}) {
		t.Fatalf("newer page = %#v, %v", newer, err)
	}
	items, err = repository.ListSummaries(t.Context(), logging.Query{
		StartAt: "2026-09-22T08:00:00.000000001+08:00", EndAt: "2026-09-22T00:00:00.000000001Z",
	})
	if err != nil || !equalStrings(ids(items), []string{"offset"}) || items[0].Timestamp != "2026-09-22T00:00:00.000000001Z" {
		t.Fatalf("exact range = %#v, %v", items, err)
	}
	stored, err := repository.readQ.GetLogSummary(t.Context(), "offset")
	if err != nil || stored.Ts != time.Date(2026, 9, 22, 0, 0, 0, 1, time.UTC).UnixNano() {
		t.Fatalf("timestamp was not stored as Unix nanoseconds: %#v, %v", stored, err)
	}
}

func TestPruneUsesExactNanosecondCutoff(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, row := range []struct{ id, timestamp string }{
		{"expired", "2026-09-21T23:59:59.999Z"},
		{"expired-submillisecond", "2026-09-21T23:59:59.999999999Z"},
		{"offset-boundary", "2026-09-22T05:30:00.000000000+05:30"},
		{"nanosecond-boundary", "2026-09-22T00:00:00.000000001Z"},
		{"after-boundary", "2026-09-22T00:00:00.1Z"},
	} {
		if err := repository.SaveSummary(t.Context(), logging.Summary{LogID: row.id, Timestamp: row.timestamp, Level: "info", Source: "fixture", Message: row.id}); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := time.Date(2026, 9, 22, 0, 0, 0, 1, time.UTC)
	if err := repository.PruneOlderThan(t.Context(), cutoff); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"expired", "expired-submillisecond", "offset-boundary"} {
		if _, err := repository.GetSummary(t.Context(), id); !errors.Is(err, logging.ErrLogNotFound) {
			t.Fatalf("expired record %s retained: %v", id, err)
		}
	}
	for _, id := range []string{"nanosecond-boundary", "after-boundary"} {
		if _, err := repository.GetSummary(t.Context(), id); err != nil {
			t.Fatalf("boundary record %s removed: %v", id, err)
		}
	}
}

func TestLogWriteRejectsUnrepresentableTimestamps(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, timestamp := range []string{"now", "subsec", "invalid", "9998-01-01T00:00:00Z"} {
		if err := repository.SaveSummary(t.Context(), logging.Summary{LogID: timestamp, Timestamp: timestamp, Level: "info", Source: "fixture", Message: "fixture"}); err == nil {
			t.Fatalf("accepted invalid timestamp %q", timestamp)
		}
	}
	items, err := repository.ListSummaries(t.Context(), logging.Query{Limit: 10})
	if err != nil || len(items) != 0 {
		t.Fatalf("invalid timestamps persisted: %+v, %v", items, err)
	}
}

func TestTimeFiltersOutsideNanosecondRangeKeepTheirMeaning(t *testing.T) {
	repository := openLoggingRepository(t)
	if err := repository.SaveSummary(t.Context(), logging.Summary{LogID: "fixture", Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		start, end string
		count      int
	}{
		{"0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z", 1},
		{"0001-01-01T00:00:00Z", "1600-01-01T00:00:00Z", 0},
		{"2300-01-01T00:00:00Z", "9999-12-31T23:59:59Z", 0},
	} {
		items, err := repository.ListPage(t.Context(), logging.PageQuery{StartAt: tc.start, EndAt: tc.end})
		if err != nil || len(items.Items) != tc.count {
			t.Fatalf("range %s..%s = %+v, %v", tc.start, tc.end, items, err)
		}
	}
}
