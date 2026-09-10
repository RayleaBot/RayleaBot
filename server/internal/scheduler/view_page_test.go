package scheduler

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
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
