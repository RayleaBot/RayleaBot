package sqlite

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestMigration000007PreservesLogOrderingFiltersAndCursors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	legacy, err := os.ReadFile("../../../storage/testdata/schema-000001.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Other tables are unchanged by 000007; use the historical log table with
	// the exact 000006 expression indexes and version marker.
	schema := string(legacy)
	start := strings.Index(schema, "CREATE TABLE IF NOT EXISTS management_logs")
	end := strings.Index(schema[start:], "CREATE TABLE IF NOT EXISTS plugin_kv") + start
	if _, err := db.Exec(schema[start:end] + `CREATE TABLE schema_metadata(singleton_id INTEGER PRIMARY KEY, version TEXT NOT NULL, initialized_at TEXT NOT NULL);
INSERT INTO schema_metadata VALUES(1,'000006','2026-09-13T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	for name, prefix := range map[string]string{"ts": "", "plugin": "plugin_id, ", "request": "request_id, ", "source": "source, ", "boot_ts": "boot_id, "} {
		if _, err := db.Exec("DROP INDEX idx_management_logs_" + name + "; CREATE INDEX idx_management_logs_" + name + " ON management_logs(" + prefix + storage.LogTimestampExpression + " DESC, id DESC)"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE INDEX idx_management_logs_prune ON management_logs ((CASE WHEN ts GLOB '[0-9]*' THEN julianday(ts) ELSE -1 END))`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id    string
		value any
	}{
		{"julian-midnight", 2451544.5}, {"julian-noon", 2451545},
		{"whole", "2026-09-22T00:00:00Z"}, {"nano-1", "2026-09-22T08:00:00.000000001+08:00"},
		{"nano-2", "2026-09-22T00:00:00.000000002Z"}, {"tied", "2026-09-21T20:00:00.000000002-04:00"},
		{"fraction", "2026-09-22T00:00:00.1Z"},
	} {
		if _, err := db.Exec(`INSERT INTO management_logs(log_id,ts,level,source,message) VALUES(?,?,'info','fixture','fixture')`, row.id, row.value); err != nil {
			t.Fatal(err)
		}
	}
	readIDs := func(query string, args ...any) []string {
		t.Helper()
		rows, err := db.Query(query, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var result []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			result = append(result, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	expression := storage.LogTimestampExpression
	beforeOrder := readIDs(`SELECT log_id FROM management_logs ORDER BY ` + expression + ` DESC, id DESC`)
	beforeExact := readIDs(`SELECT log_id FROM management_logs WHERE `+expression+` >= ? AND `+expression+` <= ? ORDER BY `+expression+` DESC,id DESC`, "2026-09-22T00:00:00.000000001Z", "2026-09-22T00:00:00.000000002Z")
	beforeJulian := readIDs(`SELECT log_id FROM management_logs WHERE julianday(ts) >= julianday('2000-01-01T00:00:00Z') AND julianday(ts) <= julianday('2000-01-01T12:00:00Z') ORDER BY julianday(ts) DESC,id DESC`)
	var rowID int64
	if err := db.QueryRow(`SELECT id FROM management_logs WHERE log_id='tied'`).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	boundary := "2026-09-22T00:00:00.000000002Z"
	beforeOlder := readIDs(`SELECT log_id FROM management_logs WHERE `+expression+` < ? OR (`+expression+` = ? AND id < ?) ORDER BY `+expression+` DESC,id DESC LIMIT 2`, boundary, boundary, rowID)
	beforeNewer := readIDs(`SELECT log_id FROM management_logs WHERE `+expression+` > ? OR (`+expression+` = ? AND id > ?) ORDER BY `+expression+` ASC,id ASC LIMIT 2`, boundary, boundary, rowID)
	oldCursor, _ := json.Marshal(map[string]any{"v": 1, "row_id": rowID, "ts": boundary})
	_ = db.Close()
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository, err := NewRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(items []logging.Summary) []string {
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.LogID)
			if len(item.Timestamp) != 30 || !strings.HasSuffix(item.Timestamp, "Z") {
				t.Fatalf("noncanonical migrated timestamp: %s", item.Timestamp)
			}
		}
		return result
	}
	all, err := repository.ListPage(t.Context(), logging.PageQuery{Limit: 20})
	if err != nil || !slices.Equal(ids(all.Items), beforeOrder) {
		t.Fatalf("migration changed order: %+v %v, want %v", all, err, beforeOrder)
	}
	exact, err := repository.ListPage(t.Context(), logging.PageQuery{StartAt: "2026-09-22T08:00:00.000000001+08:00", EndAt: boundary})
	if err != nil || !slices.Equal(ids(exact.Items), beforeExact) {
		t.Fatalf("migration changed exact filter: %+v %v, want %v", exact, err, beforeExact)
	}
	julian, err := repository.ListPage(t.Context(), logging.PageQuery{StartAt: "2000-01-01T00:00:00Z", EndAt: "2000-01-01T12:00:00Z"})
	if err != nil || !slices.Equal(ids(julian.Items), beforeJulian) {
		t.Fatalf("migration changed Julian instants: %+v %v, want %v", julian, err, beforeJulian)
	}
	for _, tc := range []struct {
		direction logging.PageDirection
		want      []string
	}{{logging.PageDirectionOlder, beforeOlder}, {logging.PageDirectionNewer, beforeNewer}} {
		page, err := repository.ListPage(t.Context(), logging.PageQuery{Limit: 2, Cursor: base64.RawURLEncoding.EncodeToString(oldCursor), Direction: tc.direction})
		if err != nil || !slices.Equal(ids(page.Items), tc.want) {
			t.Fatalf("migration changed %s cursor: %+v %v, want %v", tc.direction, page, err, tc.want)
		}
	}
}
