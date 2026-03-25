package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

var migrationNamePattern = regexp.MustCompile(`^([0-9]{4})_([a-z0-9_]+)\.sql$`)

type migration struct {
	ID                string
	Name              string
	Checksum          string
	AcceptedChecksums map[string]struct{}
	SQL               string
}

func applyMigrations(ctx context.Context, db *sql.DB, migrationFS fs.FS) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations(migrationFS)
	if err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	for _, item := range migrations {
		if checksum, ok := applied[item.ID]; ok {
			if !item.acceptsChecksum(checksum) {
				return fmt.Errorf("migration %s checksum changed", item.ID)
			}
			continue
		}

		if err := applySingleMigration(ctx, db, item); err != nil {
			return err
		}
	}

	return nil
}

func (m migration) acceptsChecksum(checksum string) bool {
	if checksum == m.Checksum {
		return true
	}

	_, ok := m.AcceptedChecksums[checksum]
	return ok
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	const statement = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at TEXT NOT NULL
);`

	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func applySingleMigration(ctx context.Context, db *sql.DB, item migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", item.ID, err)
	}

	if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", item.ID, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (id, name, checksum, applied_at) VALUES (?, ?, ?, ?)`,
		item.ID,
		item.Name,
		item.Checksum,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", item.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", item.ID, err)
	}

	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var id string
		var checksum string
		if err := rows.Scan(&id, &checksum); err != nil {
			return nil, fmt.Errorf("scan schema_migrations row: %w", err)
		}
		applied[id] = checksum
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations rows: %w", err)
	}

	return applied, nil
}

func loadMigrations(migrationFS fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	items := make([]migration, 0, len(entries))
	seen := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := migrationNamePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}

		id := matches[1]
		if previous, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate migration id %s in %s and %s", id, previous, entry.Name())
		}
		seen[id] = entry.Name()

		script, err := fs.ReadFile(migrationFS, path.Clean(entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		trimmed := strings.TrimSpace(string(script))
		if trimmed == "" {
			return nil, fmt.Errorf("migration %s is empty", entry.Name())
		}

		checksum, acceptedChecksums := buildMigrationChecksums(script)
		items = append(items, migration{
			ID:                id,
			Name:              entry.Name(),
			Checksum:          checksum,
			AcceptedChecksums: acceptedChecksums,
			SQL:               string(script),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})

	return items, nil
}

func buildMigrationChecksums(script []byte) (string, map[string]struct{}) {
	accepted := make(map[string]struct{}, 3)
	add := func(content []byte) string {
		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])
		accepted[checksum] = struct{}{}
		return checksum
	}

	normalizedLF := normalizeMigrationLineEndings(script, "\n")
	canonical := add([]byte(normalizedLF))
	add(script)
	add([]byte(normalizeMigrationLineEndings(script, "\r\n")))
	return canonical, accepted
}

func normalizeMigrationLineEndings(script []byte, newline string) string {
	normalized := strings.ReplaceAll(string(script), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if newline == "\n" {
		return normalized
	}

	return strings.ReplaceAll(normalized, "\n", newline)
}
