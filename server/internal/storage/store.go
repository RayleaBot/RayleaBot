package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	sqliteDriverName             = "sqlite"
	defaultBusyTimeout           = 5 * time.Second
	defaultReadMaxConns          = 4
	defaultWALAutoCheckpointPage = 1000
)

//go:embed schema.sql
var schemaFS embed.FS

type Option func(*options) error

type options struct {
	busyTimeout time.Duration
}

type Store struct {
	Path  string
	Read  *sql.DB
	Write *sql.DB
	lock  *dbFileLock
}

func WithBusyTimeout(timeout time.Duration) Option {
	return func(opts *options) error {
		if timeout <= 0 {
			return errors.New("busy timeout must be positive")
		}
		opts.busyTimeout = timeout
		return nil
	}
}

func Open(path string, opts ...Option) (*Store, error) {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	options := options{
		busyTimeout: defaultBusyTimeout,
	}

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite parent directory: %w", err)
	}

	lock, err := acquireDBFileLock(databaseLockPath(path))
	if err != nil {
		return nil, err
	}

	store, err := openWithProtection(path, options, lock)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	return store, nil
}

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

func (s *Store) Close() error {
	if s == nil {
		return nil
	}

	var closeErr error
	if s.Read != nil {
		if err := s.Read.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close sqlite read handle: %w", err))
		}
		s.Read = nil
	}
	if s.Write != nil {
		if err := checkpointAndTruncate(context.Background(), s.Write); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("checkpoint sqlite WAL: %w", err))
		}
		if err := s.Write.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close sqlite write handle: %w", err))
		}
		s.Write = nil
	}
	if s.lock != nil {
		if err := s.lock.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("release sqlite lock: %w", err))
		}
		s.lock = nil
	}

	return closeErr
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

func checkpointAndTruncate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func databaseLockPath(databasePath string) string {
	return databasePath + ".lock"
}

func databaseFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

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
