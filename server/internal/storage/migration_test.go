package storage

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func createLegacyDatabase(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../tests/prototypes/conversation/testdata/schema-000001.sql")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(string(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO schema_metadata VALUES (1,'000001','2026-09-13T00:00:00Z'); INSERT INTO plugin_kv VALUES ('fixture','cursor','42',2,'2026-09-13T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenMigratesLegacyAndRetainsRestorableCopy(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000002" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("metadata changed: %#v %v", metadata, err)
	}
	var value string
	var expiry sql.NullInt64
	if err := store.Read.QueryRow("SELECT value_json,expires_at_ms FROM plugin_kv WHERE plugin_id='fixture'").Scan(&value, &expiry); err != nil || value != "42" || expiry.Valid {
		t.Fatalf("legacy value changed: %q %v %v", value, expiry, err)
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(readSQLiteSchemaShape(t, store.Read), readSQLiteSchemaShape(t, fresh.Read)) {
		t.Fatal("migrated structure differs from fresh schema")
	}
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated sqlite_master differ")
	}
	copies, err := filepath.Glob(path + ".pre-migration-000001-*.db")
	if err != nil || len(copies) != 1 {
		t.Fatalf("copies=%v err=%v", copies, err)
	}
	if err := QuickCheckPath(t.Context(), copies[0]); err != nil {
		t.Fatal(err)
	}
	if version, err := ReadSchemaVersion(t.Context(), copies[0]); err != nil || version != "000001" {
		t.Fatalf("copy version=%s err=%v", version, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	after, _ := filepath.Glob(path + ".pre-migration-000001-*.db")
	if !reflect.DeepEqual(copies, after) {
		t.Fatal("repeated startup repeated migration")
	}
}

func migrationSchemaSQL(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT type,name,coalesce(sql,'') FROM sqlite_master ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var kind, name, statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatal(err)
		}
		result[kind+":"+name] = strings.ReplaceAll(strings.Join(strings.Fields(statement), ""), "IFNOTEXISTS", "")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOpenFailedMigrationDoesNotChangeLegacyDatabase(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	// The index conflict fails after ADD COLUMN, exercising DDL rollback.
	if _, err := db.Exec("CREATE INDEX idx_plugin_kv_expiry ON plugin_kv(key)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("faulty migration succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed startup changed original database")
	}
	copies, _ := filepath.Glob(path + ".pre-migration-000001-*.db")
	if len(copies) != 1 {
		t.Fatal("verified backup not retained after migration failure")
	}
	if version, err := ReadSchemaVersion(t.Context(), copies[0]); err != nil || version != "000001" {
		t.Fatal("backup not readable with old structure")
	}
}

func TestMigrationBackupFailureAndUnknownVersionDoNotWrite(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrateSchema(t.Context(), db, filepath.Join(t.TempDir(), "missing", "state.db"), "000001", schemaMigrations()); err == nil {
		t.Fatal("migration proceeded without a backup")
	}
	var version string
	if err := db.QueryRow("SELECT version FROM schema_metadata").Scan(&version); err != nil || version != "000001" {
		t.Fatal("backup failure mutated version")
	}
	if _, err := db.Exec("UPDATE schema_metadata SET version='000003'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("unknown version accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("unknown version changed database")
	}
	copies, _ := filepath.Glob(path + ".pre-migration-*")
	if len(copies) != 0 {
		t.Fatal("unknown version created migration artifacts")
	}
}
