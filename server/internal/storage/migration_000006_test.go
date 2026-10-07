package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func createSchema000005(t *testing.T) string {
	t.Helper()
	path := createSchema000004(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(logTimeIndexesSchema + `UPDATE schema_metadata SET version='000005';`); err != nil {
		t.Fatal(err)
	}
	return path
}

type kvMigrationRow struct {
	PluginID, Key, ValueJSON, SizeType, UpdatedAt string
	Size                                          any
	Expiry                                        sql.NullInt64
}

func readKVMigrationRows(t *testing.T, db *sql.DB) []kvMigrationRow {
	t.Helper()
	rows, err := db.Query(`SELECT plugin_id,key,value_json,size_bytes,typeof(size_bytes),updated_at,expires_at_ms FROM plugin_kv ORDER BY plugin_id,key`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var result []kvMigrationRow
	for rows.Next() {
		var row kvMigrationRow
		if err := rows.Scan(&row.PluginID, &row.Key, &row.ValueJSON, &row.Size, &row.SizeType, &row.UpdatedAt, &row.Expiry); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestKVIndexMigrationPreservesRowsAndInitialization(t *testing.T) {
	t.Parallel()
	path := createSchema000005(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	for index, size := range []any{int64(0), int64(65536), int64(-1), 1.5, []byte("2"), "historical-size", int64(9223372036854775807)} {
		var expiry any
		if index%2 == 1 {
			expiry = int64(1800000000000 + index)
		}
		if _, err := db.Exec(`INSERT INTO plugin_kv(plugin_id,key,value_json,size_bytes,updated_at,expires_at_ms) VALUES('fixture',?,?,?,?,?)`, fmt.Sprint(index), `"`+strings.Repeat("x", 4096)+`"`, size, "2026-10-03T00:00:00Z", expiry); err != nil {
			t.Fatal(err)
		}
	}
	before := readKVMigrationRows(t, db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if !reflect.DeepEqual(before, readKVMigrationRows(t, store.Read)) {
		t.Fatal("KV migration changed historical row values, types or deadlines")
	}
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000008" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("migration metadata = %+v, %v", metadata, err)
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated KV indexes differ")
	}
}

func TestKVIndexMigrationConflictRestoresOldSchema(t *testing.T) {
	t.Parallel()
	path := createSchema000005(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE INDEX idx_plugin_kv_size_anomaly ON plugin_kv(key)`); err != nil {
		t.Fatal(err)
	}
	before := migrationSchemaSQL(t, db)
	rows := readKVMigrationRows(t, db)
	if err := initializeSchema(t.Context(), db); err == nil {
		t.Fatal("conflicting index did not stop migration")
	}
	var version string
	if err := db.QueryRow(`SELECT version FROM schema_metadata`).Scan(&version); err != nil || version != "000005" {
		t.Fatalf("failed migration version = %q, %v", version, err)
	}
	if !reflect.DeepEqual(before, migrationSchemaSQL(t, db)) || !reflect.DeepEqual(rows, readKVMigrationRows(t, db)) {
		t.Fatal("failed migration retained partial indexes or changed rows")
	}
}

func TestKVIndexMigrationCancellationRollsBackCompletedDDL(t *testing.T) {
	t.Parallel()
	path := createSchema000005(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	before := migrationSchemaSQL(t, db)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	steps := []schemaMigration{{from: "000005", to: "000006", sql: kvMetadataIndexesSchema,
		apply: func(ctx context.Context, tx *sql.Tx) error {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE name='idx_plugin_kv_metadata'`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("new index was not created before cancellation: %d %v", count, err)
			}
			cancel()
			return ctx.Err()
		}}}
	if err := migrateSchema(ctx, db, "000005", append(steps, schemaMigrations()[5:]...)); !errors.Is(err, context.Canceled) {
		t.Fatalf("migration cancellation = %v", err)
	}
	var version string
	if err := db.QueryRow(`SELECT version FROM schema_metadata`).Scan(&version); err != nil || version != "000005" {
		t.Fatalf("cancelled migration version = %q, %v", version, err)
	}
	if !reflect.DeepEqual(before, migrationSchemaSQL(t, db)) {
		t.Fatal("cancelled migration changed the previous indexes")
	}
}
