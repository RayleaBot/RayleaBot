package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestAccessListMigrationPreservesLegacyRulesAndWhitelistState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range sortedSchemaMigrations() {
		if migration.version >= 8 {
			break
		}
		data, err := migrationFS.ReadFile(migration.file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			t.Fatalf("migration %d: %v", migration.version, err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)", migration.version, migration.name, "2026-09-09T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"blacklist_entries", "whitelist_entries"} {
		if _, err := db.Exec("INSERT INTO " + table + " (entry_type,target_id,reason,created_at) VALUES ('user','1001','keep reason','2026-01-01T00:00:00Z')"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("UPDATE whitelist_state SET enabled = 1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	var count, enabled int
	if err := store.Read.QueryRow("SELECT COUNT(*) FROM access_list_entries WHERE source_protocol='onebot11' AND source_adapter='' AND bot_id='' AND target_id='1001' AND reason='keep reason' AND created_at='2026-01-01T00:00:00Z'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("migrated entries=%d", count)
	}
	if err := store.Read.QueryRow("SELECT enabled FROM whitelist_state WHERE singleton_id=1").Scan(&enabled); err != nil || enabled != 1 {
		t.Fatalf("whitelist state=%d error=%v", enabled, err)
	}
}
