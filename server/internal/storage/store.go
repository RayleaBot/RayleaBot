package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	sqliteDriverName    = "sqlite"
	defaultBusyTimeout  = 5 * time.Second
	defaultReadMaxConns = 4
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
	}
	if s.Write != nil {
		if err := s.Write.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close sqlite write handle: %w", err))
		}
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

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("set busy_timeout: %w", err)
	}

	return nil
}

func initializeSchema(ctx context.Context, db *sql.DB) error {
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}

	if _, err := db.ExecContext(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("apply embedded schema: %w", err)
	}

	return nil
}
