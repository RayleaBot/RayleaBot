package sqlite

import (
	"testing"

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
