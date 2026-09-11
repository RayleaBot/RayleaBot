package plugins

import (
	"slices"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

func TestListPageFiltersAndPages(t *testing.T) {
	t.Parallel()

	snapshots := []Snapshot{
		{PluginID: "b-official", Name: "Official", Valid: true, RegistrationState: "installed", RuntimeState: "running", PackageSourceType: "catalog", PackageSourceRef: "official"},
		{PluginID: "a-crashed", Name: "Crashed", Valid: true, RegistrationState: "installed", RuntimeState: "crashed", PackageSourceType: "catalog", PackageSourceRef: "community"},
		{PluginID: "c-invalid", Name: "Invalid", Valid: false, RegistrationState: "installed"},
		{PluginID: "d-conflict", Name: "Conflict", Description: "shares a trigger", Valid: true, RegistrationState: "installed", RuntimeState: "running"},
	}
	conflicts := map[string][]string{"d-conflict": {"/hello"}}
	ids := func(items []Snapshot) []string {
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.PluginID)
		}
		return result
	}

	for _, test := range []struct {
		name   string
		filter ListFilter
		query  pagination.Query
		want   []string
		total  int
		next   string
	}{
		{name: "all ordered by id", query: pagination.Query{Limit: 100}, want: []string{"a-crashed", "b-official", "c-invalid", "d-conflict"}, total: 4},
		{name: "alert selects failed invalid and conflicts", filter: ListFilter{State: "alert"}, query: pagination.Query{Limit: 100}, want: []string{"a-crashed", "c-invalid", "d-conflict"}, total: 3},
		{name: "exact state", filter: ListFilter{State: PluginStateRunning}, query: pagination.Query{Limit: 100}, want: []string{"b-official", "d-conflict"}, total: 2},
		{name: "official source", filter: ListFilter{Source: "official"}, query: pagination.Query{Limit: 100}, want: []string{"b-official"}, total: 1},
		{name: "community source", filter: ListFilter{Source: "community"}, query: pagination.Query{Limit: 100}, want: []string{"a-crashed", "c-invalid", "d-conflict"}, total: 3},
		{name: "text matches description", query: pagination.Query{Limit: 100, Text: "TRIGGER"}, want: []string{"d-conflict"}, total: 1},
		{name: "page cursor", query: pagination.Query{Limit: 2}, want: []string{"a-crashed", "b-official"}, total: 4, next: "2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, meta := ListPage(snapshots, conflicts, test.filter, test.query)
			if got := ids(page); !slices.Equal(got, test.want) {
				t.Fatalf("page = %v, want %v", got, test.want)
			}
			if meta.Total != test.total || meta.NextCursor != test.next {
				t.Fatalf("meta = %+v, want total %d next %q", meta, test.total, test.next)
			}
		})
	}
}
