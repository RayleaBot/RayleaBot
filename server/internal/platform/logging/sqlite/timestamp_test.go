package sqlite

import (
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func TestMixedHistoricalTimestampsKeepExactOrderAndCursorBoundaries(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, row := range []struct{ id, timestamp string }{
		{"current", "2026-09-22T00:00:00.000000002Z"},
		{"whole", "2026-09-22T00:00:00Z"},
		{"variable", "2026-09-22T00:00:00.1Z"},
		{"offset", "2026-09-22T08:00:00.000000001+08:00"},
	} {
		// 直接写入历史形式，避免写入端归一化掩盖兼容性缺陷。
		if err := repository.writeQ.InsertLogSummary(t.Context(), sqlcgen.InsertLogSummaryParams{
			LogID: row.id, Ts: row.timestamp, Level: "info", Source: "fixture", Message: row.id, DetailsJson: "{}",
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
	if err != nil || stored.Ts != "2026-09-22T08:00:00.000000001+08:00" {
		t.Fatalf("reading rewrote historical timestamp: %#v, %v", stored, err)
	}
}

func TestPruneKeepsExistingTimestampAndCutoffPrecision(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, row := range []struct{ id, timestamp string }{
		{"expired", "2026-09-21T23:59:59.999Z"},
		{"expired-submillisecond", "2026-09-21T23:59:59.999999999Z"},
		{"offset-boundary", "2026-09-22T05:30:00.000000000+05:30"},
		{"nanosecond-boundary", "2026-09-22T00:00:00.000000001Z"},
		{"after-boundary", "2026-09-22T00:00:00.1Z"},
	} {
		if err := repository.writeQ.InsertLogSummary(t.Context(), sqlcgen.InsertLogSummaryParams{LogID: row.id, Ts: row.timestamp, Level: "info", Source: "fixture", Message: row.id, DetailsJson: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := time.Date(2026, 9, 22, 0, 0, 0, 987654321, time.UTC)
	if err := repository.PruneOlderThan(t.Context(), cutoff); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"expired", "expired-submillisecond"} {
		if _, err := repository.GetSummary(t.Context(), id); !errors.Is(err, logging.ErrLogNotFound) {
			t.Fatalf("expired record %s retained: %v", id, err)
		}
	}
	for _, id := range []string{"offset-boundary", "nanosecond-boundary", "after-boundary"} {
		if _, err := repository.GetSummary(t.Context(), id); err != nil {
			t.Fatalf("boundary record %s removed: %v", id, err)
		}
	}
}

func TestSpecialTimestampTextRemainsWritableAndPrunable(t *testing.T) {
	repository := openLoggingRepository(t)
	for _, timestamp := range []string{"now", "NOW", "subsec", "subsecond", "now\x00ignored", "invalid"} {
		if err := repository.SaveSummary(t.Context(), logging.Summary{LogID: timestamp, Timestamp: timestamp, Level: "info", Source: "fixture", Message: "fixture"}); err != nil {
			t.Fatalf("previously accepted timestamp %q rejected: %v", timestamp, err)
		}
	}
	items, err := repository.ListSummaries(t.Context(), logging.Query{Limit: 10})
	if err != nil || len(items) != 6 {
		t.Fatalf("special timestamps unreadable: %+v, %v", items, err)
	}
	if err := repository.PruneOlderThan(t.Context(), time.Date(9998, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	items, err = repository.ListSummaries(t.Context(), logging.Query{Limit: 10})
	if err != nil || len(items) != 1 || items[0].LogID != "invalid" {
		t.Fatalf("special timestamp pruning changed: %+v, %v", items, err)
	}
}
