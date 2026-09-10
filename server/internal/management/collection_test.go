package management

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestPluginListFiltersWholeCatalogBeforeReturningBoundedPages(t *testing.T) {
	snapshots := make([]plugins.Snapshot, 0, 105)
	for index := range 105 {
		snapshot := plugins.Snapshot{PluginID: fmt.Sprintf("plugin-%03d", index), Name: fmt.Sprintf("Plugin %03d", index), Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"}
		if index >= 103 {
			snapshot.Commands = []plugins.Command{{ID: "duplicate", Name: "duplicate"}}
		}
		if index == 104 {
			snapshot.PackageSourceType = "catalog"
			snapshot.PackageSourceRef = "official"
		}
		snapshots = append(snapshots, snapshot)
	}
	router := pluginRouter(t, plugincatalog.New(snapshots))
	for _, tc := range []struct {
		query        string
		total, items int
		next, id     string
	}{
		{"", 105, 100, "100", "plugin-000"},
		{"?cursor=100", 105, 5, "", "plugin-100"},
		{"?query=Plugin%20104&limit=1", 1, 1, "", "plugin-104"},
		{"?state=alert&limit=1&cursor=1", 2, 1, "", "plugin-104"},
		{"?source=official", 1, 1, "", "plugin-104"},
		{"?source=community&cursor=100", 104, 4, "", "plugin-100"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins"+tc.query, nil))
			if recorder.Code != 200 {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			body := decodeBody(t, recorder.Body.Bytes())
			items := body["items"].([]any)
			if body["total"] != float64(tc.total) || len(items) != tc.items || items[0].(map[string]any)["id"] != tc.id {
				t.Fatalf("page=%#v", body)
			}
			next, _ := body["next_cursor"].(string)
			if next != tc.next {
				t.Fatalf("next=%q", next)
			}
		})
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?cursor=-1", "?state=anything", "?source=local", "?state=running&state=disabled"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins"+query, nil))
		if recorder.Code != 400 {
			t.Fatalf("query %s returned %d", query, recorder.Code)
		}
	}
}
