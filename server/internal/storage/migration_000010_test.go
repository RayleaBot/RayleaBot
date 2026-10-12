package storage

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestBotAccountMigrationStartsEmptyAndEnforcesIdentityAndConnectionKeys(t *testing.T) {
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range schemaMigrations() {
		if step.to == "000010" {
			break
		}
		if err := WithTx(t.Context(), db, nil, func(tx *sql.Tx) error {
			if step.sql != "" {
				if _, err := tx.Exec(step.sql); err != nil {
					return err
				}
			}
			if step.apply != nil {
				if err := step.apply(t.Context(), tx); err != nil {
					return err
				}
			}
			_, err := tx.Exec("UPDATE schema_metadata SET version = ?", step.to)
			return err
		}); err != nil {
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
	var count int
	if err := store.Read.QueryRow("SELECT COUNT(*) FROM bot_accounts").Scan(&count); err != nil || count != 0 {
		t.Fatalf("migrated directory = %d, %v", count, err)
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated schemas differ")
	}
	insert := func(protocol, bot string, connection any) error {
		_, err := store.Write.Exec(`INSERT INTO bot_accounts
			(source_protocol, bot_id, adapter_id, adapter_display_name, last_adapter_id, last_adapter_display_name, last_seen_at)
			VALUES (?, ?, ?, 'OneBot11', 'onebot', 'OneBot11', '2026-10-12T00:00:00Z')`, protocol, bot, connection)
		return err
	}
	if err := insert("onebot11", "10001", "onebot"); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		protocol, bot string
		connection    any
	}{
		{"onebot11", "10001", nil},
		{"onebot11", "20002", "onebot"},
		{"unknown", "30003", nil},
		{"onebot11", "", nil},
	} {
		if err := insert(row.protocol, row.bot, row.connection); err == nil {
			t.Fatalf("accepted conflicting or invalid row: %+v", row)
		}
	}
	for _, id := range []string{"20002", "30003"} {
		if err := insert("onebot11", id, nil); err != nil {
			t.Fatal(err)
		}
	}
}
