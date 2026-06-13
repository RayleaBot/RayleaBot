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
