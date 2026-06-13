package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
)

func openWithProtection(path string, options options, lock *dbFileLock) (*Store, error) {
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

	store, err := openConfigured(path, options, lock)
	if err == nil {
		return store, nil
	}
	if databaseFileExists(path) && isSQLiteCorruptionError(err) {
		if quarantineErr := quarantineMalformedDatabase(path, err); quarantineErr != nil {
			return nil, quarantineErr
		}
		return openConfigured(path, options, lock)
	}
	return nil, err
}

func openConfigured(path string, options options, lock *dbFileLock) (*Store, error) {
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

	if err := configureHandle(context.Background(), writeDB, options.busyTimeout); err != nil {
		return cleanup(fmt.Errorf("configure sqlite write handle: %w", err))
	}
	if err := configureHandle(context.Background(), readDB, options.busyTimeout); err != nil {
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

func configureHandle(ctx context.Context, db *sql.DB, busyTimeout time.Duration) error {
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

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds())); err != nil {
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
