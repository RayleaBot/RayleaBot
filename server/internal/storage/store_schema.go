package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"slices"
	"time"
)

// currentSchemaVersion identifies the structure in schema.sql.
const currentSchemaVersion = "000002"

type schemaMigration struct{ from, to, sql string }

func schemaMigrations() []schemaMigration {
	return []schemaMigration{{from: "000001", to: "000002", sql: `
ALTER TABLE plugin_kv ADD COLUMN expires_at_ms INTEGER;
CREATE INDEX idx_plugin_kv_expiry ON plugin_kv(expires_at_ms) WHERE expires_at_ms IS NOT NULL;`}}
}

// SupportedSchemaVersions is the forward-migratable set, oldest first.
func SupportedSchemaVersions() []string {
	versions := make([]string, 0, len(schemaMigrations())+1)
	for _, migration := range schemaMigrations() {
		versions = append(versions, migration.from)
	}
	return append(versions, currentSchemaVersion)
}

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
		if _, err := time.Parse(time.RFC3339Nano, initializedAt); err != nil {
			return fmt.Errorf("invalid database initialization timestamp: %w", err)
		}
		if version == currentSchemaVersion {
			return nil
		}
		if !slices.Contains(SupportedSchemaVersions(), version) {
			return fmt.Errorf("database schema version does not match the supported migration chain: %s", version)
		}
		return migrateSchema(ctx, db, version, schemaMigrations())
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

// Runs before connection pragmas can change an old database. Each registered
// step has one transaction and preserves the original initialization timestamp.
func migrateSchema(ctx context.Context, db *sql.DB, source string, steps []schemaMigration) error {
	var chain []schemaMigration
	version := source
	for _, step := range steps {
		if step.from == version {
			chain = append(chain, step)
			version = step.to
		}
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("no forward migration from schema %s", source)
	}
	for _, step := range chain {
		if err := WithTx(ctx, db, nil, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, step.sql); err != nil {
				return err
			}
			result, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET version = ? WHERE singleton_id = 1 AND version = ?", step.to, step.from)
			if err != nil {
				return err
			}
			count, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("migration source version changed")
			}
			return nil
		}); err != nil {
			return fmt.Errorf("migrate schema %s to %s: %w", step.from, step.to, err)
		}
	}
	slog.Info("数据库结构迁移完成", "component", "storage", "source_version", source, "target_version", currentSchemaVersion)
	return nil
}
