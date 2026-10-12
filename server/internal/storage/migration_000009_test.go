package storage

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestAccountListMigrationKeepsEarliestEntries(t *testing.T) {
	t.Parallel()
	path := createSchema000007(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	// Prepare the exact predecessor through the existing session migration.
	for _, step := range schemaMigrations() {
		if step.from == "000007" {
			if _, err := db.Exec(step.sql + `UPDATE schema_metadata SET version = '000008';`); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, list := range []string{"blacklist", "whitelist"} {
		if _, err := db.Exec(`INSERT INTO access_list_entries
			(list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id, reason, created_at) VALUES
			(?, 'onebot11', '', '', 'user', 'target', 'global', '2026-10-01T00:00:00Z'),
			(?, 'qqofficial', 'first', 'bot-a', 'user', 'target', 'later', '2026-10-03T00:00:00Z'),
			(?, 'qqofficial', 'replacement', 'bot-a', 'user', 'target', 'earliest', '2026-10-01T00:00:00Z'),
			(?, 'qqofficial', 'tie', 'bot-a', 'user', 'target', 'same time', '2026-10-01T00:00:00Z'),
			(?, 'qqofficial', 'first', 'bot-b', 'user', 'target', 'other bot', '2026-10-01T00:00:00Z'),
			(?, 'onebot11', 'first', 'bot-a', 'user', 'target', 'other protocol', '2026-10-01T00:00:00Z'),
			(?, 'qqofficial', 'first', 'bot-a', 'group', 'target', 'group', '2026-10-01T00:00:00Z'),
			(?, 'qqofficial', 'first', 'bot-a', 'user', 'other', 'other target', '2026-10-01T00:00:00Z')`,
			list, list, list, list, list, list, list, list); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for index, list := range []string{"blacklist", "whitelist"} {
		var count, id int
		var reason, createdAt string
		if err := store.Read.QueryRow(`SELECT count(*) FROM access_list_entries WHERE list_kind = ?`, list).Scan(&count); err != nil || count != 6 {
			t.Fatalf("%s count = %d, %v", list, count, err)
		}
		if err := store.Read.QueryRow(`SELECT id, reason, created_at FROM access_list_entries
			WHERE list_kind = ? AND source_protocol = 'qqofficial' AND bot_id = 'bot-a' AND entry_type = 'user' AND target_id = 'target'`, list).Scan(&id, &reason, &createdAt); err != nil || id != index*8+3 || reason != "earliest" || createdAt != "2026-10-01T00:00:00Z" {
			t.Fatalf("%s retained row = %d %q %q, %v", list, id, reason, createdAt, err)
		}
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated schemas differ")
	}
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000009" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("metadata = %+v, %v", metadata, err)
	}
}
