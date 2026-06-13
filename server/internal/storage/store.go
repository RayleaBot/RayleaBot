package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
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
