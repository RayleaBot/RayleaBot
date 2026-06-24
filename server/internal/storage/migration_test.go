package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyMigrationsConvergeToCurrentSchemaShape(t *testing.T) {
	t.Parallel()

	current := openTestStore(t)

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	createLegacySchemaDatabase(t, legacyPath)
	legacy := mustOpenStore(t, legacyPath)
	defer legacy.Close()

	currentShape := readSQLiteSchemaShape(t, current.Read)
	legacyShape := readSQLiteSchemaShape(t, legacy.Read)
	if !reflect.DeepEqual(legacyShape, currentShape) {
		currentJSON, _ := json.MarshalIndent(currentShape, "", "  ")
		legacyJSON, _ := json.MarshalIndent(legacyShape, "", "  ")
		t.Fatalf("legacy migration schema shape drifted from current schema\ncurrent:\n%s\nlegacy:\n%s", currentJSON, legacyJSON)
	}
}

func createLegacySchemaDatabase(t *testing.T, databasePath string) {
	t.Helper()

	db, err := sql.Open(sqliteDriverName, databasePath)
	if err != nil {
		t.Fatalf("open fixture sqlite: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fixture sqlite: %v", err)
		}
	}()

	if _, err := db.Exec(`CREATE TABLE third_party_accounts (
    platform TEXT NOT NULL CHECK (platform IN ('bilibili')),
    account_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    secret_key TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (platform, account_id)
)`); err != nil {
		t.Fatalf("create legacy third_party_accounts: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE bilibili_source_rooms (
    uid TEXT PRIMARY KEY,
    room_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    face TEXT NOT NULL DEFAULT '',
    live_status INTEGER NOT NULL DEFAULT 0 CHECK (live_status IN (0, 1)),
    live_started_at INTEGER NOT NULL DEFAULT 0,
    live_event_id TEXT NOT NULL DEFAULT '',
    connection_state TEXT NOT NULL DEFAULT 'idle' CHECK (connection_state IN ('idle', 'connecting', 'connected', 'degraded', 'failed')),
    last_event_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
)`); err != nil {
		t.Fatalf("create legacy bilibili_source_rooms: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO third_party_accounts (platform, account_id, label, enabled, secret_key, updated_at) VALUES ('bilibili', 'primary', '主账号', 1, 'third_party:bilibili:primary:cookie', '2026-06-08T08:00:00Z')`,
	); err != nil {
		t.Fatalf("insert legacy third-party account: %v", err)
	}
}

type sqliteSchemaShape struct {
	Tables  map[string]sqliteTableShape `json:"tables"`
	Indexes map[string]string           `json:"indexes"`
}

type sqliteTableShape struct {
	Columns map[string]sqliteColumnShape `json:"columns"`
}

type sqliteColumnShape struct {
	Type       string `json:"type"`
	NotNull    int    `json:"not_null"`
	Default    string `json:"default"`
	PrimaryKey int    `json:"primary_key"`
}

func readSQLiteSchemaShape(t *testing.T, db *sql.DB) sqliteSchemaShape {
	t.Helper()

	shape := sqliteSchemaShape{
		Tables:  map[string]sqliteTableShape{},
		Indexes: map[string]string{},
	}

	for _, tableName := range readTables(t, db) {
		if strings.HasPrefix(tableName, "sqlite_") {
			continue
		}
		shape.Tables[tableName] = readSQLiteTableShape(t, db, tableName)
	}

	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'index' AND sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("query sqlite indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatalf("scan sqlite index: %v", err)
		}
		shape.Indexes[name] = strings.Join(strings.Fields(sql), " ")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite indexes: %v", err)
	}

	return shape
}

func readSQLiteTableShape(t *testing.T, db *sql.DB, tableName string) sqliteTableShape {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + quoteSQLiteIdentifier(tableName) + `)`)
	if err != nil {
		t.Fatalf("query table info for %s: %v", tableName, err)
	}
	defer rows.Close()

	table := sqliteTableShape{Columns: map[string]sqliteColumnShape{}}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info for %s: %v", tableName, err)
		}
		table.Columns[name] = sqliteColumnShape{
			Type:       strings.ToUpper(strings.TrimSpace(columnType)),
			NotNull:    notNull,
			Default:    sqliteSchemaValue(defaultValue),
			PrimaryKey: primaryKey,
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info for %s: %v", tableName, err)
	}
	return table
}

func quoteSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func sqliteSchemaValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}
