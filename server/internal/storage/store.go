package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

const (
	sqliteDriverName             = "sqlite"
	defaultBusyTimeout           = 5 * time.Second
	defaultReadMaxConns          = 4
	defaultWALAutoCheckpointPage = 1000
)

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

func checkpointAndTruncate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func CurrentSchemaVersion() string {
	return fmt.Sprintf("%06d", latestSchemaMigrationVersion())
}

func latestSchemaMigrationVersion() int {
	latest := 0
	for _, migration := range schemaMigrations {
		if migration.version > latest {
			latest = migration.version
		}
	}
	return latest
}

type AppliedMigration struct {
	Version   int
	Name      string
	AppliedAt string
}

func (s *Store) ListAppliedMigrations(ctx context.Context) ([]AppliedMigration, error) {
	if s == nil || s.Read == nil {
		return nil, errors.New("sqlite store is required")
	}
	rows, err := sqlcgen.New(s.Read).ListSchemaMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schema migrations: %w", err)
	}
	items := make([]AppliedMigration, 0, len(rows))
	for _, row := range rows {
		items = append(items, AppliedMigration{
			Version:   int(row.Version),
			Name:      row.Name,
			AppliedAt: row.AppliedAt,
		})
	}
	return items, nil
}
