package scheduler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

func TestJobPagesApplyStatusSearchAndSortBeforeLimit(t *testing.T) {
	older := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	engine := &Engine{jobs: map[string]Job{
		"first":  {JobID: "first", PluginID: "a", LastRun: &older, LastDurationMS: 30},
		"second": {JobID: "second", PluginID: "b", LastRun: &newer, LastDurationMS: 10, LastError: &RunError{Code: "plugin.internal_error", Message: "fixture"}},
		"never":  {JobID: "never", PluginID: "c"},
	}}
	view, err := NewView(engine, func(id string) string { return "Owner " + id })
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		status, sort, query, id string
		total                   int
	}{
		{"", "duration", "", "first", 3}, {"", "last_run", "", "second", 3},
		{"success", "name", "", "first", 2}, {"error", "name", "", "second", 1},
		{"", "name", "OWNER C", "never", 1},
	} {
		page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1, Text: tc.query}, Status: tc.status, Sort: tc.sort})
		if page.Total != tc.total || len(page.Items) != 1 || page.Items[0].JobID != tc.id {
			t.Fatalf("%+v: %#v", tc, page)
		}
	}
}

func TestJobPagesKeepPayloadSnapshotDuringUpdates(t *testing.T) {
	repository, err := NewSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Options{Repository: repository, Logger: slog.New(slog.NewTextHandler(discardWriter{}, nil)), Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	upsert := func(i int) error {
		_, err := engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "0 9 * * *", json.RawMessage(fmt.Sprintf(`{"target_id":%d,"content":"%d"}`, i, i)))
		return err
	}
	if err := upsert(0); err != nil {
		t.Fatal(err)
	}
	view, err := NewView(engine, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 1; i <= 32; i++ {
			if err := upsert(i); err != nil {
				t.Error(err)
				return
			}
		}
	})
	for i := 0; i < 100; i++ {
		page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1}})
		if len(page.Items) != 1 || page.Items[0].PayloadSummary.TargetID != page.Items[0].PayloadSummary.Content {
			t.Errorf("inconsistent payload: %+v", page)
			break
		}
	}
	wg.Wait()
}

func TestJobPagesPreserveDisplaySearchAndSecondPrecisionOrdering(t *testing.T) {
	older := time.Date(2026, 9, 10, 1, 0, 0, 100, time.UTC)
	newer := older.Add(800 * time.Millisecond)
	engine := &Engine{location: time.UTC, jobs: map[string]Job{
		"z": {JobID: "z", PluginID: "a", LastRun: &older, LastDurationMS: 10, Payload: json.RawMessage(`{"target_type":"group","target_id":9007199254740993,"content":"  ","summary":"Needle summary"}`)},
		"a": {JobID: "a", PluginID: "b", LastRun: &newer, LastDurationMS: 10, LastError: &RunError{}},
		"x": {JobID: "x", PluginID: "c", LogLabel: "  Label  ", LastError: &RunError{Code: "fixture.error", At: older}, Payload: json.RawMessage(`{"content":true}`)},
		"y": {JobID: "y", PluginID: "d", Payload: json.RawMessage(`invalid`)},
	}}
	names := map[string]string{"a": "Zulu", "b": "alpha", "c": "Alpha", "d": "   "}
	view, err := NewView(engine, func(id string) string { return names[id] })
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sort, status, text string
		cursor                   int
		want                     []string
		total                    int
	}{
		{"name", "", "", "", 0, []string{"a", "x", "y", "z"}, 4},
		{"seconds", "last_run", "", "", 0, []string{"z", "a", "x", "y"}, 4},
		{"duration", "duration", "", "", 1, []string{"a", "x", "y"}, 4},
		{"empty-error", "duration", "success", "", 0, []string{"z", "a", "y"}, 3},
		{"error", "", "error", "", 0, []string{"x"}, 1},
		{"payload", "", "", " NEEDLE ", 0, []string{"z"}, 1},
		{"bool", "", "", "TrUe", 0, []string{"x"}, 1},
		{"label", "", "", "LABEL", 0, []string{"x"}, 1},
		{"owner", "", "", "ALPHA", 0, []string{"a", "x"}, 2},
		{"past-end", "", "", "", 8, []string{}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 10, Cursor: tc.cursor, Text: tc.text}, Sort: tc.sort, Status: tc.status})
			ids := make([]string, 0, len(page.Items))
			for _, item := range page.Items {
				ids = append(ids, item.JobID)
			}
			if page.Total != tc.total || !slices.Equal(ids, tc.want) {
				t.Fatalf("page = %+v, want %v total %d", page, tc.want, tc.total)
			}
		})
	}
	page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1, Text: "Needle"}})
	if page.Items[0].PayloadSummary.ConversationID != "group:9007199254740993" {
		t.Fatal(page.Items[0].PayloadSummary)
	}
	*page.Items[0].LastRun = "changed"
	names["a"] = "Aardvark"
	page = view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1}})
	if page.Items[0].JobID != "z" || page.Items[0].PluginName != "Aardvark" || *page.Items[0].LastRun != older.Format(time.RFC3339) {
		t.Fatalf("snapshot or name stale: %+v", page)
	}
}
