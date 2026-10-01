package storage

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestMessageStatsMigrationFrom000003(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schemaMigrations()[0].sql + `UPDATE schema_metadata SET version='000003';`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("migration and fresh schema differ")
	}
	var count int
	if err := store.Read.QueryRow("SELECT COUNT(*) FROM message_stats_tracking").Scan(&count); err != nil || count != 0 {
		t.Fatalf("offline migration started tracking: %d %v", count, err)
	}
}

func TestMessageStatsMigrationRollback(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schemaMigrations()[0].sql + `UPDATE schema_metadata SET version='000003'; CREATE TABLE message_stats_runs(conflict TEXT);`); err != nil {
		t.Fatal(err)
	}
	if err = initializeSchema(t.Context(), db); err == nil {
		t.Fatal("expected migration conflict")
	}
	var version string
	if err := db.QueryRow("SELECT version FROM schema_metadata").Scan(&version); err != nil || version != "000003" {
		t.Fatalf("version = %s, %v", version, err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='message_stats_hours'").Scan(&count); err != nil || count != 0 {
		t.Fatal("partial migration retained")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
