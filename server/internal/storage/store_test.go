package storage

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestOpenBootstrapsSQLiteWithExpectedPragmas(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	if store.Read == nil || store.Write == nil {
		t.Fatalf("expected read/write handles to be initialized")
	}
	if store.Read == store.Write {
		t.Fatalf("expected distinct read/write handles")
	}
	if got := store.Write.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("unexpected write max open connections: got %d want 1", got)
	}

	assertPragmaString(t, store.Write, "journal_mode", "wal")
	assertPragmaString(t, store.Read, "journal_mode", "wal")
	assertPragmaInt(t, store.Write, "busy_timeout", int(defaultBusyTimeout.Milliseconds()))
	assertPragmaInt(t, store.Read, "busy_timeout", int(defaultBusyTimeout.Milliseconds()))
	assertTableExists(t, store.Read, "schema_migrations")
	assertTableExists(t, store.Read, "auth_bootstrap_state")
	assertTableExists(t, store.Read, "admin_sessions")
	assertTableExists(t, store.Read, "plugin_instances")
	assertTableExists(t, store.Read, "plugin_packages")
	assertTableExists(t, store.Read, "plugin_grants")
	assertTableExists(t, store.Read, "tasks")
	assertTableExists(t, store.Read, "secret_store")
	assertTableExists(t, store.Read, "scheduler_jobs")
	assertTableExists(t, store.Read, "blacklist_entries")
	assertTableExists(t, store.Read, "management_logs")
	assertTableExists(t, store.Read, "plugin_kv")
	assertTableExists(t, store.Read, "system_configs")
	assertTableExists(t, store.Read, "render_template_revisions")
	assertTableExists(t, store.Read, "render_template_states")
	assertColumnExists(t, store.Read, "management_logs", "log_id")
	assertColumnExists(t, store.Read, "management_logs", "details_json")
	assertColumnExists(t, store.Read, "management_logs", "boot_id")
	assertColumnExists(t, store.Read, "render_template_revisions", "source_digest")
	assertColumnExists(t, store.Read, "render_template_states", "validation_issue_count")
	assertColumnExists(t, store.Read, "plugin_grants", "expires_at")
	assertIndexExists(t, store.Read, "idx_management_logs_log_id")
	assertIndexExists(t, store.Read, "idx_management_logs_boot_ts")
	assertIndexExists(t, store.Read, "idx_management_logs_source")
	assertIndexExists(t, store.Read, "idx_plugin_grants_expires_at")
	assertIndexExists(t, store.Read, "idx_plugin_kv_plugin_id")
	assertIndexExists(t, store.Read, "idx_system_configs_namespace")
	assertIndexExists(t, store.Read, "idx_render_template_revisions_template_saved_at")
	assertIndexExists(t, store.Read, "idx_render_template_revisions_template_digest")

	tables := readTables(t, store.Read)
	if len(tables) != 16 {
		t.Fatalf("unexpected table set: %#v", tables)
	}
}

func TestOpenAppliesMigrationsOnlyOnce(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	store := mustOpenStore(t, databasePath)
	store.Close()

	second := mustOpenStore(t, databasePath)
	defer second.Close()

	var count int
	if err := second.Read.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations rows: %v", err)
	}
	if count != 18 {
		t.Fatalf("unexpected migration count: got %d want 18", count)
	}
}

func TestLoadMigrationsRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	_, err := loadMigrations(fstest.MapFS{
		"0001_auth.sql": {Data: []byte("CREATE TABLE t1 (id INTEGER);")},
		"0001_more.sql": {Data: []byte("CREATE TABLE t2 (id INTEGER);")},
	})
	if err == nil {
		t.Fatalf("expected duplicate migration id error")
	}
}

func TestOpenAcceptsEquivalentMigrationWithDifferentLineEndings(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	lfMigrations := fstest.MapFS{
		"0001_test.sql": {Data: []byte("CREATE TABLE test_items (\n\tid INTEGER PRIMARY KEY\n);\n")},
	}
	crlfMigrations := fstest.MapFS{
		"0001_test.sql": {Data: []byte("CREATE TABLE test_items (\r\n\tid INTEGER PRIMARY KEY\r\n);\r\n")},
	}

	store := mustOpenStoreWithMigrations(t, databasePath, lfMigrations)
	if err := store.Close(); err != nil {
		t.Fatalf("close LF store: %v", err)
	}

	reopened, err := Open(databasePath, WithMigrationsFS(crlfMigrations))
	if err != nil {
		t.Fatalf("Open with CRLF-equivalent migration failed: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("close CRLF store: %v", err)
		}
	})
}

func TestOpenAcceptsEquivalentMigrationWhenStoredChecksumWasCRLF(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	lfMigrations := fstest.MapFS{
		"0001_test.sql": {Data: []byte("CREATE TABLE test_items (\n\tid INTEGER PRIMARY KEY\n);\n")},
	}
	crlfMigrations := fstest.MapFS{
		"0001_test.sql": {Data: []byte("CREATE TABLE test_items (\r\n\tid INTEGER PRIMARY KEY\r\n);\r\n")},
	}

	store := mustOpenStoreWithMigrations(t, databasePath, crlfMigrations)
	if err := store.Close(); err != nil {
		t.Fatalf("close CRLF store: %v", err)
	}

	reopened, err := Open(databasePath, WithMigrationsFS(lfMigrations))
	if err != nil {
		t.Fatalf("Open with LF-equivalent migration failed: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("close LF store: %v", err)
		}
	})
}

func TestOpenUpgradesExistingAuthDatabaseToPluginStateTables(t *testing.T) {
	t.Parallel()

	authOnlyFS := readAuthOnlyMigrations(t)
	databasePath := filepath.Join(t.TempDir(), "state.db")

	store := mustOpenStoreWithMigrations(t, databasePath, authOnlyFS)
	if _, err := store.Write.Exec(
		`INSERT INTO auth_bootstrap_state (singleton_id, identifier, secret_digest, signing_key, initialized_at)
		 VALUES (1, ?, ?, ?, ?)`,
		"admin",
		[]byte("digest"),
		[]byte("signing-key"),
		"2026-03-20T09:00:00Z",
	); err != nil {
		t.Fatalf("seed bootstrap state: %v", err)
	}
	if _, err := store.Write.Exec(
		`INSERT INTO admin_sessions (session_id, subject, issued_at, expires_at)
		 VALUES (?, ?, ?, ?)`,
		"sess_1",
		"admin",
		"2026-03-20T09:00:00Z",
		"2026-03-21T09:00:00Z",
	); err != nil {
		t.Fatalf("seed admin session: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close auth-only store: %v", err)
	}

	upgraded := mustOpenStore(t, databasePath)
	defer upgraded.Close()

	assertTableExists(t, upgraded.Read, "plugin_instances")
	assertTableExists(t, upgraded.Read, "plugin_packages")

	var migrationCount int
	if err := upgraded.Read.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("count schema_migrations rows: %v", err)
	}
	if migrationCount != 18 {
		t.Fatalf("unexpected migration count after upgrade: got %d want 18", migrationCount)
	}

	var bootstrapCount int
	if err := upgraded.Read.QueryRow(`SELECT COUNT(*) FROM auth_bootstrap_state`).Scan(&bootstrapCount); err != nil {
		t.Fatalf("count auth_bootstrap_state rows: %v", err)
	}
	if bootstrapCount != 1 {
		t.Fatalf("unexpected bootstrap row count: got %d want 1", bootstrapCount)
	}

	var sessionCount int
	if err := upgraded.Read.QueryRow(`SELECT COUNT(*) FROM admin_sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("count admin_sessions rows: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("unexpected session row count: got %d want 1", sessionCount)
	}
}

func TestPluginInstancesRejectsDuplicateIDsAndInvalidDesiredState(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	if _, err := store.Write.Exec(
		`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES (?, ?, ?)`,
		"weather",
		"enabled",
		"2026-03-20T09:00:00Z",
	); err != nil {
		t.Fatalf("insert initial plugin instance: %v", err)
	}

	if _, err := store.Write.Exec(
		`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES (?, ?, ?)`,
		"weather",
		"disabled",
		"2026-03-20T09:05:00Z",
	); err == nil {
		t.Fatalf("expected duplicate plugin_id insert to fail")
	}

	if _, err := store.Write.Exec(
		`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES (?, ?, ?)`,
		"clock",
		"paused",
		"2026-03-20T09:10:00Z",
	); err == nil {
		t.Fatalf("expected invalid desired_state insert to fail")
	}
}

func TestPluginPackagesRejectInvalidSourceType(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	if _, err := store.Write.Exec(
		`INSERT INTO plugin_packages (
			plugin_id,
			source_type,
			source_ref,
			version,
			manifest_hash,
			package_hash,
			installed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"weather",
		"remote_zip",
		"https://example.invalid/weather.zip",
		"0.1.0",
		"manifest",
		"package",
		"2026-03-20T09:00:00Z",
	); err == nil {
		t.Fatalf("expected invalid source_type insert to fail")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	store := mustOpenStore(t, filepath.Join(t.TempDir(), "state.db"))
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})
	return store
}

func mustOpenStore(t *testing.T, path string) *Store {
	t.Helper()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%s) failed: %v", path, err)
	}
	return store
}

func mustOpenStoreWithMigrations(t *testing.T, path string, migrations fs.FS) *Store {
	t.Helper()

	store, err := Open(path, WithMigrationsFS(migrations))
	if err != nil {
		t.Fatalf("Open(%s) with custom migrations failed: %v", path, err)
	}
	return store
}

func readAuthOnlyMigrations(t *testing.T) fs.FS {
	t.Helper()

	script, err := fs.ReadFile(embeddedMigrations, "migrations/0001_auth_core.sql")
	if err != nil {
		t.Fatalf("read embedded 0001_auth_core.sql: %v", err)
	}

	return fstest.MapFS{
		"0001_auth_core.sql": {Data: script},
	}
}

func assertPragmaString(t *testing.T, db *sql.DB, name, want string) {
	t.Helper()

	var got string
	if err := db.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
		t.Fatalf("query PRAGMA %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("unexpected PRAGMA %s: got %q want %q", name, got, want)
	}
}

func assertPragmaInt(t *testing.T, db *sql.DB, name string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
		t.Fatalf("query PRAGMA %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("unexpected PRAGMA %s: got %d want %d", name, got, want)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&exists); err != nil {
		t.Fatalf("query sqlite_master for %s: %v", name, err)
	}
	if exists != 1 {
		t.Fatalf("expected table %s to exist", name)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, tableName, columnName string) {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		t.Fatalf("query table info for %s: %v", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info row: %v", err)
		}
		if name == columnName {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info rows: %v", err)
	}
	t.Fatalf("expected column %s.%s to exist", tableName, columnName)
}

func assertIndexExists(t *testing.T, db *sql.DB, indexName string) {
	t.Helper()

	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&exists); err != nil {
		t.Fatalf("query sqlite_master for index %s: %v", indexName, err)
	}
	if exists != 1 {
		t.Fatalf("expected index %s to exist", indexName)
	}
}

func readTables(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatalf("query sqlite_master tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite_master row: %v", err)
		}
		tables = append(tables, name)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master rows: %v", err)
	}

	return tables
}
