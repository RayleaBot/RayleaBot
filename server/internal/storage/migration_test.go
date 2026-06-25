package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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

func TestCurrentSchemaSnapshotMatchesNewDatabaseShape(t *testing.T) {
	t.Parallel()

	current := openTestStore(t)
	snapshotPath := filepath.Join(t.TempDir(), "snapshot.db")
	createCurrentSchemaSnapshotDatabase(t, snapshotPath)
	snapshot, err := sql.Open(sqliteDriverName, snapshotPath)
	if err != nil {
		t.Fatalf("open snapshot sqlite: %v", err)
	}
	defer snapshot.Close()

	currentShape := readSQLiteSchemaShape(t, current.Read)
	snapshotShape := readSQLiteSchemaShape(t, snapshot)
	if !reflect.DeepEqual(snapshotShape, currentShape) {
		currentJSON, _ := json.MarshalIndent(currentShape, "", "  ")
		snapshotJSON, _ := json.MarshalIndent(snapshotShape, "", "  ")
		t.Fatalf("current schema snapshot drifted from new database schema\ncurrent:\n%s\nsnapshot:\n%s", currentJSON, snapshotJSON)
	}
}

func TestSQLCUsesCurrentSchemaSnapshot(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "..", "sqlc.yaml"))
	if err != nil {
		t.Fatalf("read sqlc.yaml: %v", err)
	}
	if !strings.Contains(string(content), `schema: "internal/storage/schema.sql"`) {
		t.Fatalf("sqlc.yaml must use internal/storage/schema.sql as the current schema source")
	}
}

func TestLegacyBaseDoesNotContainCompatibilityColumns(t *testing.T) {
	t.Parallel()

	payload, err := migrationFS.ReadFile("migrations/000001_base.sql")
	if err != nil {
		t.Fatalf("read legacy base migration: %v", err)
	}
	base := string(payload)
	for _, fragment := range []string{
		"profile_uid",
		"profile_nickname",
		"profile_avatar_url",
		"credential_state",
		"credential_checked_at",
		"credential_last_error",
		"last_used_at",
		"proxy_url",
		"proxy_enabled",
		"cover_url",
		"'weibo'",
		"'douyin'",
		"'netease_music'",
	} {
		if strings.Contains(base, fragment) {
			t.Fatalf("legacy base migration contains compatibility fragment %q", fragment)
		}
	}
}

func createCurrentSchemaSnapshotDatabase(t *testing.T, databasePath string) {
	t.Helper()

	db, err := sql.Open(sqliteDriverName, databasePath)
	if err != nil {
		t.Fatalf("open snapshot sqlite: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close snapshot sqlite: %v", err)
		}
	}()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin snapshot schema transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := execMigrationSQL(t.Context(), tx, currentSchemaSQL); err != nil {
		t.Fatalf("apply current schema snapshot: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit snapshot schema transaction: %v", err)
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
