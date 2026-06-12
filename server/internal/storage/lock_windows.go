//go:build windows

package storage

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
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

	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
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
	var overlapped windows.Overlapped
	if err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &overlapped); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("unlock sqlite database: %w", err))
	}
	if err := l.file.Close(); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("close sqlite lock file: %w", err))
	}
	l.file = nil
	return closeErr
}
