package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"
)

// currentSchemaVersion identifies the structure in schema.sql.
const currentSchemaVersion = "000001"

//go:embed schema.sql
var currentSchemaSQL string

func CurrentSchemaVersion() string { return currentSchemaVersion }

// ReadSchemaVersion reads the archived database's own metadata without modifying it.
func ReadSchemaVersion(ctx context.Context, path string) (string, error) {
	db, err := sql.Open(sqliteDriverName, sqliteReadOnlyDSN(path))
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	var version string
	if err := db.QueryRowContext(ctx, "SELECT version FROM schema_metadata WHERE singleton_id = 1").Scan(&version); err != nil {
		return "", fmt.Errorf("read database schema metadata: %w", err)
	}
	return version, nil
}

func initializeSchema(ctx context.Context, db *sql.DB) error {
	var hasMetadata int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_metadata'").Scan(&hasMetadata); err != nil {
		return fmt.Errorf("inspect schema metadata: %w", err)
	}
	if hasMetadata != 0 {
		var version, initializedAt string
		if err := db.QueryRowContext(ctx, "SELECT version, initialized_at FROM schema_metadata WHERE singleton_id = 1").Scan(&version, &initializedAt); err != nil {
			return fmt.Errorf("read schema metadata: %w", err)
		}
		if version != currentSchemaVersion {
			return fmt.Errorf("database schema version does not match the current structure")
		}
		if _, err := time.Parse(time.RFC3339Nano, initializedAt); err != nil {
			return fmt.Errorf("invalid database initialization timestamp: %w", err)
		}
		return nil
	}
	var tableCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount); err != nil {
		return fmt.Errorf("inspect empty database: %w", err)
	}
	if tableCount != 0 {
		return fmt.Errorf("database is not empty and has no initialization metadata")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database initialization: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, currentSchemaSQL); err != nil {
		return fmt.Errorf("create current database structure: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_metadata (singleton_id, version, initialized_at) VALUES (1, ?, ?)", currentSchemaVersion, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record database initialization: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database initialization: %w", err)
	}
	return nil
}
