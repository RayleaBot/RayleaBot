package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

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
