package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func createSchema000007(t *testing.T) string {
	t.Helper()
	schema, err := os.ReadFile("testdata/schema-000007.sql")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(string(schema) + `
INSERT INTO schema_metadata VALUES (1, '000007', '2026-09-13T00:00:00Z');
INSERT INTO auth_bootstrap_state VALUES (1, 'admin', X'01', X'02', '2026-09-13T00:00:00Z');
INSERT INTO admin_sessions VALUES ('legacy-session', 'admin', '2026-10-07T00:00:00Z', '2026-10-08T00:00:00Z');
INSERT INTO secret_store VALUES ('platform.auth.session_signing_key', X'02', '2026-09-13T00:00:00Z', '2026-09-13T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpaqueSessionMigrationMatchesFreshSchema(t *testing.T) {
	t.Parallel()
	store, err := Open(createSchema000007(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated schemas differ")
	}
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000008" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("metadata = %+v, %v", metadata, err)
	}
	// Signed sessions cannot be validated without their key, so the migration ends them and drops the key.
	var sessions, keys int
	if err := store.Read.QueryRow("SELECT count(*) FROM admin_sessions").Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("legacy sessions = %d, %v", sessions, err)
	}
	if err := store.Read.QueryRow("SELECT count(*) FROM secret_store WHERE key = 'platform.auth.session_signing_key'").Scan(&keys); err != nil || keys != 0 {
		t.Fatalf("legacy signing secrets = %d, %v", keys, err)
	}
}
