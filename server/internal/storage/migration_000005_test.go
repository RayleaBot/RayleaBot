package storage

import (
	"database/sql"
	"reflect"
	"testing"
)

func createSchema000004(t *testing.T) string {
	t.Helper()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(schemaMigrations()[0].sql + messageStatsSchema + `UPDATE schema_metadata SET version='000004';`); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLogIndexMigrationPreservesHistoricalRows(t *testing.T) {
	t.Parallel()
	path := createSchema000004(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	timestamps := []string{
		"2026-10-03T00:00:00.000000001Z", "2026-10-03T00:00:00Z", "2026-10-03T00:00:00.1Z",
		"2026-10-03T05:30:00.000000002+05:30", "now", "subsec", "invalid", "now\x00ignored",
	}
	for index, timestamp := range timestamps {
		if _, err := db.Exec(`INSERT INTO management_logs(id,log_id,ts,level,source,message,details_json) VALUES(?,? ,?,'info','fixture','original','{"value":"original"}')`, index+1, index+1, timestamp); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(logTimeIndexesSchema); err != nil {
		t.Fatal(err)
	}
	for index, want := range timestamps {
		var timestamp, message, details string
		if err := db.QueryRow(`SELECT ts,message,details_json FROM management_logs WHERE id=?`, index+1).Scan(&timestamp, &message, &details); err != nil || timestamp != want || message != "original" || details != `{"value":"original"}` {
			t.Fatalf("historical row %d changed: %q %q %q, %v", index, timestamp, message, details, err)
		}
	}
}

func TestLogIndexMigrationFailureRestoresOldIndexes(t *testing.T) {
	t.Parallel()
	path := createSchema000004(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE INDEX idx_management_logs_prune ON management_logs(id)`); err != nil {
		t.Fatal(err)
	}
	before := migrationSchemaSQL(t, db)
	if err := initializeSchema(t.Context(), db); err == nil {
		t.Fatal("conflicting index did not stop migration")
	}
	var version string
	if err := db.QueryRow(`SELECT version FROM schema_metadata`).Scan(&version); err != nil || version != "000004" {
		t.Fatalf("failed migration version = %q, %v", version, err)
	}
	if !reflect.DeepEqual(before, migrationSchemaSQL(t, db)) {
		t.Fatal("failed migration changed existing indexes")
	}
}
