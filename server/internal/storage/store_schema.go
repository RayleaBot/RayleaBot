package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed schema.sql
var schemaFS embed.FS

func initializeSchema(ctx context.Context, db *sql.DB) error {
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}

	if _, err := db.ExecContext(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("apply embedded schema: %w", err)
	}

	if err := ensureThirdPartyAccountColumns(ctx, db); err != nil {
		return err
	}
	if err := ensureThirdPartyAccountPlatforms(ctx, db); err != nil {
		return err
	}
	if err := ensureBilibiliSourceRoomColumns(ctx, db); err != nil {
		return err
	}

	return nil
}

func ensureThirdPartyAccountColumns(ctx context.Context, db *sql.DB) error {
	columns := []string{
		"profile_uid TEXT NOT NULL DEFAULT ''",
		"profile_nickname TEXT NOT NULL DEFAULT ''",
		"profile_avatar_url TEXT NOT NULL DEFAULT ''",
		"credential_state TEXT NOT NULL DEFAULT 'unknown' CHECK (credential_state IN ('unknown', 'valid', 'invalid'))",
		"credential_checked_at TEXT",
		"credential_last_error TEXT NOT NULL DEFAULT ''",
		"last_used_at TEXT",
		"proxy_url TEXT NOT NULL DEFAULT ''",
		"proxy_enabled INTEGER NOT NULL DEFAULT 0 CHECK (proxy_enabled IN (0, 1))",
	}
	for _, column := range columns {
		if _, err := db.ExecContext(ctx, "ALTER TABLE third_party_accounts ADD COLUMN "+column); err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("add third_party_accounts column %q: %w", column, err)
		}
	}
	return nil
}

func ensureThirdPartyAccountPlatforms(ctx context.Context, db *sql.DB) error {
	var createSQL string
	if err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'third_party_accounts'`).Scan(&createSQL); err != nil {
		return fmt.Errorf("inspect third_party_accounts schema: %w", err)
	}
	if strings.Contains(createSQL, "'weibo'") && strings.Contains(createSQL, "'douyin'") && strings.Contains(createSQL, "'netease_music'") {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin third_party_accounts platform migration: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(ctx, `ALTER TABLE third_party_accounts RENAME TO third_party_accounts_legacy`); err != nil {
		return fmt.Errorf("rename third_party_accounts for platform migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE third_party_accounts (
    platform TEXT NOT NULL CHECK (platform IN ('bilibili', 'weibo', 'douyin', 'netease_music')),
    account_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    secret_key TEXT NOT NULL,
    profile_uid TEXT NOT NULL DEFAULT '',
    profile_nickname TEXT NOT NULL DEFAULT '',
    profile_avatar_url TEXT NOT NULL DEFAULT '',
    credential_state TEXT NOT NULL DEFAULT 'unknown' CHECK (credential_state IN ('unknown', 'valid', 'invalid')),
    credential_checked_at TEXT,
    credential_last_error TEXT NOT NULL DEFAULT '',
    last_used_at TEXT,
    proxy_url TEXT NOT NULL DEFAULT '',
    proxy_enabled INTEGER NOT NULL DEFAULT 0 CHECK (proxy_enabled IN (0, 1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (platform, account_id)
)`); err != nil {
		return fmt.Errorf("create third_party_accounts with expanded platform set: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO third_party_accounts (
    platform, account_id, label, enabled, secret_key,
    profile_uid, profile_nickname, profile_avatar_url,
    credential_state, credential_checked_at, credential_last_error,
    last_used_at, proxy_url, proxy_enabled, updated_at
)
SELECT
    platform, account_id, label, enabled, secret_key,
    profile_uid, profile_nickname, profile_avatar_url,
    credential_state, credential_checked_at, credential_last_error,
    last_used_at, proxy_url, proxy_enabled, updated_at
FROM third_party_accounts_legacy`); err != nil {
		return fmt.Errorf("copy third_party_accounts rows for platform migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE third_party_accounts_legacy`); err != nil {
		return fmt.Errorf("drop migrated third_party_accounts legacy table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_third_party_accounts_platform ON third_party_accounts (platform)`); err != nil {
		return fmt.Errorf("create third_party_accounts platform index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit third_party_accounts platform migration: %w", err)
	}
	return nil
}

func ensureBilibiliSourceRoomColumns(ctx context.Context, db *sql.DB) error {
	columns := []string{
		"cover_url TEXT NOT NULL DEFAULT ''",
	}
	for _, column := range columns {
		if _, err := db.ExecContext(ctx, "ALTER TABLE bilibili_source_rooms ADD COLUMN "+column); err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("add bilibili_source_rooms column %q: %w", column, err)
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
