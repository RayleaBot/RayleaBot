package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
)

func openWithProtection(path string, lock *filelock.Lock) (*Store, error) {
	if databaseFileExists(path) {
		if err := QuickCheckPath(context.Background(), path); err != nil {
			if !isSQLiteCorruptionError(err) {
				return nil, fmt.Errorf("check sqlite integrity: %w", err)
			}
			if err := quarantineMalformedDatabase(path, err); err != nil {
				return nil, err
			}
		}
	}

	store, err := openConfigured(path, lock)
	if err == nil {
		return store, nil
	}
	if databaseFileExists(path) && isSQLiteCorruptionError(err) {
		if quarantineErr := quarantineMalformedDatabase(path, err); quarantineErr != nil {
			return nil, quarantineErr
		}
		return openConfigured(path, lock)
	}
	return nil, err
}

func openConfigured(path string, lock *filelock.Lock) (*Store, error) {
	writeDB, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite write handle: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readDB, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("open sqlite read handle: %w", err)
	}
	readDB.SetMaxOpenConns(defaultReadMaxConns)
	readDB.SetMaxIdleConns(defaultReadMaxConns)

	cleanup := func(cause error) (*Store, error) {
		_ = readDB.Close()
		_ = writeDB.Close()
		return nil, cause
	}

	if err := configureHandle(context.Background(), writeDB); err != nil {
		return cleanup(fmt.Errorf("configure sqlite write handle: %w", err))
	}
	if err := configureHandle(context.Background(), readDB); err != nil {
		return cleanup(fmt.Errorf("configure sqlite read handle: %w", err))
	}
	if _, err := readDB.ExecContext(context.Background(), "PRAGMA query_only = ON"); err != nil {
		return cleanup(fmt.Errorf("set sqlite read handle to query_only: %w", err))
	}
	if err := initializeSchema(context.Background(), writeDB); err != nil {
		return cleanup(fmt.Errorf("initialize sqlite schema: %w", err))
	}

	return &Store{
		Path:  path,
		Read:  readDB,
		Write: writeDB,
		lock:  lock,
	}, nil
}

func configureHandle(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable foreign_keys: %w", err)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("enable WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = FULL"); err != nil {
		return fmt.Errorf("set synchronous: %w", err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA wal_autocheckpoint = %d", defaultWALAutoCheckpointPage)); err != nil {
		return fmt.Errorf("set wal_autocheckpoint: %w", err)
	}

	return nil
}

func databaseLockPath(databasePath string) string {
	return databasePath + ".lock"
}

func databaseFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
