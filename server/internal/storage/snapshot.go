package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultSnapshotRetention = 3

type snapshotFile struct {
	path    string
	modTime time.Time
	name    string
}

func SnapshotDirForDatabase(databasePath string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(databasePath)), "sqlite-snapshots")
}

func CreateSnapshot(ctx context.Context, databasePath string) (string, error) {
	databasePath = filepath.Clean(databasePath)
	if _, err := os.Stat(databasePath); err != nil {
		return "", err
	}

	db, err := sql.Open(sqliteDriverName, databasePath)
	if err != nil {
		return "", fmt.Errorf("open sqlite database for snapshot: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeout.Milliseconds())); err != nil {
		return "", fmt.Errorf("set snapshot busy_timeout: %w", err)
	}
	return createSnapshot(ctx, db, databasePath, defaultSnapshotRetention)
}

func (s *Store) CreateSnapshot(ctx context.Context) (string, error) {
	if s == nil || s.Write == nil {
		return "", fmt.Errorf("sqlite store is required")
	}
	return createSnapshot(ctx, s.Write, s.Path, defaultSnapshotRetention)
}

func createSnapshot(ctx context.Context, db *sql.DB, databasePath string, retain int) (string, error) {
	snapshotDir := SnapshotDirForDatabase(databasePath)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", fmt.Errorf("create sqlite snapshot directory: %w", err)
	}

	snapshotPath, err := nextSnapshotPath(snapshotDir, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO "+sqliteStringLiteral(snapshotPath)); err != nil {
		_ = os.Remove(snapshotPath)
		return "", fmt.Errorf("create sqlite snapshot: %w", err)
	}
	if err := QuickCheckPath(ctx, snapshotPath); err != nil {
		_ = os.Remove(snapshotPath)
		return "", fmt.Errorf("verify sqlite snapshot: %w", err)
	}
	if err := pruneSnapshots(ctx, snapshotDir, retain); err != nil {
		return "", err
	}
	return snapshotPath, nil
}

func nextSnapshotPath(snapshotDir string, now time.Time) (string, error) {
	base := "rayleabot-" + now.UTC().Format("20060102T150405.000000000Z")
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%02d", base, i)
		}
		path := filepath.Join(snapshotDir, name+".db")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", fmt.Errorf("stat sqlite snapshot path: %w", err)
		}
	}
	return "", fmt.Errorf("allocate sqlite snapshot path: too many collisions for %s", base)
}

func pruneSnapshots(ctx context.Context, snapshotDir string, retain int) error {
	if retain < 1 {
		retain = 1
	}
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return fmt.Errorf("read sqlite snapshot directory: %w", err)
	}

	snapshots := make([]snapshotFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}
		path := filepath.Join(snapshotDir, entry.Name())
		if err := QuickCheckPath(ctx, path); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat sqlite snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshotFile{
			path:    path,
			modTime: info.ModTime(),
			name:    entry.Name(),
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].modTime.Equal(snapshots[j].modTime) {
			return snapshots[i].name > snapshots[j].name
		}
		return snapshots[i].modTime.After(snapshots[j].modTime)
	})
	if len(snapshots) <= retain {
		return nil
	}
	for _, snapshot := range snapshots[retain:] {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old sqlite snapshot: %w", err)
		}
	}
	return nil
}

func sqliteStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
