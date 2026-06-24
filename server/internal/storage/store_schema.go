package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"
)

const migrationTimestampFormat = time.RFC3339Nano

//go:embed migrations/*.sql
var migrationFS embed.FS

type schemaMigration struct {
	version               int
	name                  string
	file                  string
	ignoreDuplicateColumn bool
	skip                  func(context.Context, *sql.DB) (bool, error)
}

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		name:    "base",
		file:    "migrations/000001_base.sql",
	},
	{
		version:               2,
		name:                  "add_third_party_account_columns",
		file:                  "migrations/000002_add_third_party_account_columns.sql",
		ignoreDuplicateColumn: true,
		skip:                  thirdPartyAccountColumnsMigrated,
	},
	{
		version: 3,
		name:    "expand_third_party_account_platforms",
		file:    "migrations/000003_expand_third_party_account_platforms.sql",
		skip:    thirdPartyAccountPlatformsMigrated,
	},
	{
		version:               4,
		name:                  "add_bilibili_source_room_cover_url",
		file:                  "migrations/000004_add_bilibili_source_room_cover_url.sql",
		ignoreDuplicateColumn: true,
		skip:                  bilibiliSourceRoomCoverURLMigrated,
	},
}

func initializeSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations := append([]schemaMigration(nil), schemaMigrations...)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for _, migration := range migrations {
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration schemaMigration) error {
	applied, err := migrationApplied(ctx, db, migration.version)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if migration.skip != nil {
		skip, err := migration.skip(ctx, db)
		if err != nil {
			return fmt.Errorf("inspect migration %06d %s: %w", migration.version, migration.name, err)
		}
		if skip {
			return recordMigration(ctx, db, migration.version)
		}
	}

	payload, err := migrationFS.ReadFile(migration.file)
	if err != nil {
		return fmt.Errorf("read migration %06d %s: %w", migration.version, migration.name, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %06d %s: %w", migration.version, migration.name, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := execMigrationSQL(ctx, tx, string(payload), migration.ignoreDuplicateColumn); err != nil {
		return fmt.Errorf("apply migration %06d %s: %w", migration.version, migration.name, err)
	}
	if err := recordMigrationTx(ctx, tx, migration.version); err != nil {
		return fmt.Errorf("record migration %06d %s: %w", migration.version, migration.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %06d %s: %w", migration.version, migration.name, err)
	}
	return nil
}

func execMigrationSQL(ctx context.Context, tx *sql.Tx, payload string, ignoreDuplicateColumn bool) error {
	for _, statement := range splitSQLStatements(payload) {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			if ignoreDuplicateColumn && isDuplicateColumnError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func splitSQLStatements(payload string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(payload, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}

	raw := strings.Split(cleaned.String(), ";")
	statements := make([]string, 0, len(raw))
	for _, statement := range raw {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}

func migrationApplied(ctx context.Context, db *sql.DB, version int) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("query schema_migrations version %d: %w", version, err)
	}
	return count > 0, nil
}

func recordMigration(ctx context.Context, db *sql.DB, version int) error {
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(migrationTimestampFormat))
	return err
}

func recordMigrationTx(ctx context.Context, tx *sql.Tx, version int) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(migrationTimestampFormat))
	return err
}

func thirdPartyAccountColumnsMigrated(ctx context.Context, db *sql.DB) (bool, error) {
	return tableHasColumns(ctx, db, "third_party_accounts", []string{
		"profile_uid",
		"profile_nickname",
		"profile_avatar_url",
		"credential_state",
		"credential_checked_at",
		"credential_last_error",
		"last_used_at",
		"proxy_url",
		"proxy_enabled",
	})
}

func thirdPartyAccountPlatformsMigrated(ctx context.Context, db *sql.DB) (bool, error) {
	var createSQL string
	if err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'third_party_accounts'`).Scan(&createSQL); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(createSQL, "'weibo'") &&
		strings.Contains(createSQL, "'douyin'") &&
		strings.Contains(createSQL, "'netease_music'"), nil
}

func bilibiliSourceRoomCoverURLMigrated(ctx context.Context, db *sql.DB) (bool, error) {
	return tableHasColumns(ctx, db, "bilibili_source_rooms", []string{"cover_url"})
}

func tableHasColumns(ctx context.Context, db *sql.DB, tableName string, columnNames []string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+tableName+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	present := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		present[name] = true
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	for _, columnName := range columnNames {
		if !present[columnName] {
			return false, nil
		}
	}
	return true, nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
