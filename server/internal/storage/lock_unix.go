//go:build !windows

package storage

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type dbFileLock struct {
	file *os.File
	path string
}

func acquireDBFileLock(path string) (*dbFileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open sqlite lock file: %w", err)
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, fmt.Errorf("sqlite database is already in use: %s", path)
		}
		return nil, fmt.Errorf("lock sqlite database: %w", err)
	}

	return &dbFileLock{file: file, path: path}, nil
}

func (l *dbFileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	var closeErr error
	if err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("unlock sqlite database: %w", err))
	}
	if err := l.file.Close(); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("close sqlite lock file: %w", err))
	}
	l.file = nil
	return closeErr
}
