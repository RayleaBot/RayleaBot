package storage

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func createSchema000006(t *testing.T) string {
	t.Helper()
	path := createSchema000005(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(kvMetadataIndexesSchema + `UPDATE schema_metadata SET version='000006';`); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLogNanosecondsMigrationPreservesDataAndSequence(t *testing.T) {
	path := createSchema000006(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	values := []struct {
		value any
		want  string
	}{
		{2451544.5, "2000-01-01T00:00:00Z"},
		{2451545, "2000-01-01T12:00:00Z"},
		{"2026-09-22T00:00:00.000000002Z", "2026-09-22T00:00:00.000000002Z"},
		{"2026-09-22T00:00:00Z", "2026-09-22T00:00:00Z"},
		{"2026-09-22T00:00:00.1Z", "2026-09-22T00:00:00.1Z"},
		{"2026-09-22T05:30:00.000000001+05:30", "2026-09-22T00:00:00.000000001Z"},
		{"2026-09-21 20:00:00.123456789-04:00", "2026-09-22T00:00:00.123456789Z"},
		{"2026-09-22 00:00:00.123456", "2026-09-22T00:00:00.123456Z"},
		{"1677-09-21T00:12:43.145224192Z", "1677-09-21T00:12:43.145224192Z"},
		{"2262-04-11T23:47:16.854775807Z", "2262-04-11T23:47:16.854775807Z"},
	}
	for index, row := range values {
		if _, err := db.Exec(`INSERT INTO management_logs(id,log_id,ts,level,source,message,details_json) VALUES(?, ?, ?, 'info','fixture','original','{"key":42}')`, index+1, index+1, row.value); err != nil {
			t.Fatal(err)
		}
	}
	for index, value := range []any{
		"now", "subsec", "invalid", "now\x00ignored", "9998-01-01T00:00:00Z",
		"1677-09-21T00:12:43.145224191Z", "2262-04-11T23:47:16.854775808Z",
		1721425.5, 5373483.5,
	} {
		if _, err := db.Exec(`INSERT INTO management_logs(id,log_id,ts,level,source,message) VALUES(?, ?, ?, 'info','fixture','invalid')`, index+100, index+100, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO management_logs(id,log_id,ts,level,source,message) VALUES(200,'pruned','2026-09-22T00:00:00Z','info','fixture','fixture'); DELETE FROM management_logs WHERE id=200`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for index, row := range values {
		var ns int64
		var kind, message, details string
		want, _ := time.Parse(time.RFC3339Nano, row.want)
		if err := store.Read.QueryRow(`SELECT ts,typeof(ts),message,details_json FROM management_logs WHERE id=?`, index+1).Scan(&ns, &kind, &message, &details); err != nil || ns != want.UnixNano() || kind != "integer" || message != "original" || details != `{"key":42}` {
			t.Fatalf("migrated row %d: %d %s %s %s, %v", index, ns, kind, message, details, err)
		}
	}
	var count int
	if err := store.Read.QueryRow(`SELECT count(*) FROM management_logs`).Scan(&count); err != nil || count != len(values) {
		t.Fatalf("unparseable rows retained: %d %v", count, err)
	}
	result, err := store.Write.Exec(`INSERT INTO management_logs(log_id,ts,level,source,message) VALUES('next',0,'info','fixture','next')`)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := result.LastInsertId(); err != nil || id != 201 {
		t.Fatalf("migration reused a historical row ID: %d %v", id, err)
	}
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000008" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("metadata = %+v, %v", metadata, err)
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated schemas differ")
	}
}
