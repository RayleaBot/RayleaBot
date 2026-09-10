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

	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

const (
	sqliteDriverName             = "sqlite"
	defaultBusyTimeout           = 5 * time.Second
	defaultReadMaxConns          = 4
	defaultWALAutoCheckpointPage = 1000
)

type Store struct {
	Path  string
	Read  *sql.DB
	Write *sql.DB
	lock  *filelock.Lock
}

func Open(path string) (*Store, error) {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite parent directory: %w", err)
	}

	lockPath := databaseLockPath(path)
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		if errors.Is(err, filelock.ErrLocked) {
			return nil, fmt.Errorf("sqlite database is already in use: %s", lockPath)
		}
		return nil, fmt.Errorf("lock sqlite database: %w", err)
	}

	store, err := openWithProtection(path, lock)
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

type SchemaMetadata struct {
	Version       string
	InitializedAt string
}

func (s *Store) SchemaMetadata(ctx context.Context) (SchemaMetadata, error) {
	if s.Read == nil {
		return SchemaMetadata{}, errors.New("sqlite store is required")
	}
	row, err := sqlcgen.New(s.Read).ReadSchemaMetadata(ctx)
	if err != nil {
		return SchemaMetadata{}, fmt.Errorf("read schema metadata: %w", err)
	}
	return SchemaMetadata{Version: row.Version, InitializedAt: row.InitializedAt}, nil
}
