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

func TestCurrentSchemaSnapshotMatchesNewDatabaseShape(t *testing.T) {
	t.Parallel()

	current := openTestStore(t)
	snapshotPath := filepath.Join(t.TempDir(), "snapshot.db")
	createCurrentSchemaSnapshotDatabase(t, snapshotPath)
	snapshot, err := sql.Open(sqliteDriverName, snapshotPath)
	if err != nil {
		t.Fatalf("open snapshot sqlite: %v", err)
	}
	defer func(release func() error) { _ = release() }(snapshot.Close)

	currentShape := readSQLiteSchemaShape(t, current.Read)
	snapshotShape := readSQLiteSchemaShape(t, snapshot)
	if !reflect.DeepEqual(snapshotShape, currentShape) {
		currentJSON, _ := json.MarshalIndent(currentShape, "", "  ")
		snapshotJSON, _ := json.MarshalIndent(snapshotShape, "", "  ")
		t.Fatalf("current schema snapshot drifted from new database schema\ncurrent:\n%s\nsnapshot:\n%s", currentJSON, snapshotJSON)
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
	if _, err := tx.ExecContext(t.Context(), currentSchemaSQL); err != nil {
		t.Fatalf("apply current schema snapshot: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit snapshot schema transaction: %v", err)
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
	defer func(release func() error) { _ = release() }(rows.Close)
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
	defer func(release func() error) { _ = release() }(rows.Close)

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
