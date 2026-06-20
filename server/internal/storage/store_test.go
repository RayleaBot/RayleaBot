package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	assertPragmaInt(t, store.Write, "synchronous", 2)
	assertPragmaInt(t, store.Read, "synchronous", 2)
	assertPragmaInt(t, store.Write, "foreign_keys", 1)
	assertPragmaInt(t, store.Read, "foreign_keys", 1)
	assertPragmaInt(t, store.Write, "busy_timeout", int(defaultBusyTimeout.Milliseconds()))
	assertPragmaInt(t, store.Read, "busy_timeout", int(defaultBusyTimeout.Milliseconds()))
	assertPragmaInt(t, store.Write, "wal_autocheckpoint", defaultWALAutoCheckpointPage)
	assertPragmaInt(t, store.Read, "wal_autocheckpoint", defaultWALAutoCheckpointPage)
	assertPragmaInt(t, store.Read, "query_only", 1)
	assertTableExists(t, store.Read, "auth_bootstrap_state")
	assertTableExists(t, store.Read, "admin_sessions")
	assertTableExists(t, store.Read, "plugin_instances")
	assertTableExists(t, store.Read, "plugin_packages")
	assertTableExists(t, store.Read, "tasks")
	assertTableExists(t, store.Read, "secret_store")
	assertTableExists(t, store.Read, "scheduler_jobs")
	assertTableExists(t, store.Read, "blacklist_entries")
	assertTableExists(t, store.Read, "whitelist_entries")
	assertTableExists(t, store.Read, "whitelist_state")
	assertTableExists(t, store.Read, "management_logs")
	assertTableExists(t, store.Read, "plugin_kv")
	assertTableExists(t, store.Read, "system_configs")
	assertTableExists(t, store.Read, "third_party_accounts")
	assertTableExists(t, store.Read, "bilibili_source_rooms")
	assertTableExists(t, store.Read, "bilibili_source_seen")
	assertTableExists(t, store.Read, "bilibili_source_dynamics")
	assertTableExists(t, store.Read, "bilibili_source_state")
	assertTableExists(t, store.Read, "render_template_revisions")
	assertTableExists(t, store.Read, "render_template_states")
	assertColumnExists(t, store.Read, "management_logs", "log_id")
	assertColumnExists(t, store.Read, "management_logs", "details_json")
	assertColumnExists(t, store.Read, "management_logs", "boot_id")
	assertColumnExists(t, store.Read, "scheduler_jobs", "log_label")
	assertColumnExists(t, store.Read, "render_template_revisions", "source_digest")
	assertColumnExists(t, store.Read, "render_template_states", "validation_issue_count")
	assertColumnExists(t, store.Read, "render_template_states", "source_type")
	assertColumnExists(t, store.Read, "render_template_states", "source_plugin_id")
	assertColumnExists(t, store.Read, "render_template_states", "source_local_id")
	assertColumnExists(t, store.Read, "third_party_accounts", "profile_uid")
	assertColumnExists(t, store.Read, "third_party_accounts", "profile_nickname")
	assertColumnExists(t, store.Read, "third_party_accounts", "profile_avatar_url")
	assertColumnExists(t, store.Read, "third_party_accounts", "credential_state")
	assertColumnExists(t, store.Read, "third_party_accounts", "credential_checked_at")
	assertColumnExists(t, store.Read, "third_party_accounts", "credential_last_error")
	assertColumnExists(t, store.Read, "third_party_accounts", "last_used_at")
	assertColumnExists(t, store.Read, "third_party_accounts", "proxy_url")
	assertColumnExists(t, store.Read, "third_party_accounts", "proxy_enabled")
	assertColumnExists(t, store.Read, "bilibili_source_rooms", "cover_url")
	assertIndexExists(t, store.Read, "idx_management_logs_log_id")
	assertIndexExists(t, store.Read, "idx_management_logs_boot_ts")
	assertIndexExists(t, store.Read, "idx_management_logs_source")
	assertIndexExists(t, store.Read, "idx_plugin_kv_plugin_id")
	assertIndexExists(t, store.Read, "idx_system_configs_namespace")
	assertIndexExists(t, store.Read, "idx_third_party_accounts_platform")
	assertIndexExists(t, store.Read, "idx_bilibili_source_rooms_state")
	assertIndexExists(t, store.Read, "idx_bilibili_source_seen_uid")
	assertIndexExists(t, store.Read, "idx_bilibili_source_dynamics_observed_at")
	assertIndexExists(t, store.Read, "idx_render_template_revisions_template_saved_at")
	assertIndexExists(t, store.Read, "idx_render_template_revisions_template_digest")
	assertIndexExists(t, store.Read, "idx_render_template_states_source")

	tables := readTables(t, store.Read)
	if len(tables) != 22 {
		t.Fatalf("unexpected table set: %#v", tables)
	}
	assertTableMissing(t, store.Read, "schema_migrations")
}

func TestOpenCanReopenCurrentSchemaDatabase(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	store := mustOpenStore(t, databasePath)
	store.Close()

	second := mustOpenStore(t, databasePath)
	defer second.Close()

	var bootstrapCount int
	if err := second.Read.QueryRow(`SELECT COUNT(*) FROM auth_bootstrap_state`).Scan(&bootstrapCount); err != nil {
		t.Fatalf("count auth_bootstrap_state rows: %v", err)
	}
	if bootstrapCount != 0 {
		t.Fatalf("unexpected bootstrap row count: got %d want 0", bootstrapCount)
	}
}

func TestOpenQuarantinesMalformedDatabaseAndCreatesFreshStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	databasePath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(databasePath, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(databasePath+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(databasePath+"-shm", []byte("shm"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := mustOpenStore(t, databasePath)
	defer store.Close()

	if err := QuickCheckPath(context.Background(), databasePath); err != nil {
		t.Fatalf("fresh database quick_check failed: %v", err)
	}

	quarantineRoot := filepath.Join(dir, "quarantine")
	entries, err := os.ReadDir(quarantineRoot)
	if err != nil {
		t.Fatalf("read quarantine dir: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("expected one quarantine directory, got %#v", entries)
	}

	quarantineDir := filepath.Join(quarantineRoot, entries[0].Name())
	for _, name := range []string{"state.db", "state.db-wal", "state.db-shm", "reason.json"} {
		if _, err := os.Stat(filepath.Join(quarantineDir, name)); err != nil {
			t.Fatalf("expected quarantined %s: %v", name, err)
		}
	}

	reasonPayload, err := os.ReadFile(filepath.Join(quarantineDir, "reason.json"))
	if err != nil {
		t.Fatalf("read reason.json: %v", err)
	}
	var reason quarantineReason
	if err := json.Unmarshal(reasonPayload, &reason); err != nil {
		t.Fatalf("decode reason.json: %v", err)
	}
	if reason.OriginalPath != databasePath || reason.Error == "" || len(reason.Files) != 3 {
		t.Fatalf("unexpected quarantine reason: %#v", reason)
	}
}

func TestOpenRejectsSecondHandleForSameDatabasePath(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	store := mustOpenStore(t, databasePath)
	defer store.Close()

	second, err := Open(databasePath)
	if err == nil {
		second.Close()
		t.Fatal("expected second Open for the same database path to fail")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("unexpected lock error: %v", err)
	}
}

func TestCloseCheckpointsWALAndLeavesDatabaseReadable(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	store := mustOpenStore(t, databasePath)
	if _, err := store.Write.Exec(`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES ('checkpoint', 'enabled', '2026-06-13T00:00:00Z')`); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if err := QuickCheckPath(context.Background(), databasePath); err != nil {
		t.Fatalf("database quick_check after close failed: %v", err)
	}
	if info, err := os.Stat(databasePath + "-wal"); err == nil && info.Size() != 0 {
		t.Fatalf("expected WAL file to be absent or truncated, size=%d", info.Size())
	}
}

func TestCreateSnapshotUsesValidSQLiteFileAndRetainsThree(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "state.db")
	store := mustOpenStore(t, databasePath)
	defer store.Close()

	for i := 0; i < 5; i++ {
		snapshotPath, err := store.CreateSnapshot(context.Background())
		if err != nil {
			t.Fatalf("create snapshot %d: %v", i, err)
		}
		if err := QuickCheckPath(context.Background(), snapshotPath); err != nil {
			t.Fatalf("snapshot quick_check failed: %v", err)
		}
	}

	snapshots := validSnapshotFiles(t, SnapshotDirForDatabase(databasePath))
	if len(snapshots) != defaultSnapshotRetention {
		t.Fatalf("unexpected snapshot count: got %d want %d: %#v", len(snapshots), defaultSnapshotRetention, snapshots)
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

func validSnapshotFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}
	var snapshots []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := QuickCheckPath(context.Background(), path); err != nil {
			t.Fatalf("snapshot %s failed quick_check: %v", path, err)
		}
		snapshots = append(snapshots, path)
	}
	return snapshots
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

func assertTableMissing(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&exists); err != nil {
		t.Fatalf("query sqlite_master for %s: %v", name, err)
	}
	if exists != 0 {
		t.Fatalf("expected table %s to be absent", name)
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
