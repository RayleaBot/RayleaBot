package storage

import (
	"database/sql"
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

	tables := readTables(t, store.Read)
	if len(tables) != 3 {
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
	if count != 1 {
		t.Fatalf("unexpected migration count: got %d want 1", count)
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
