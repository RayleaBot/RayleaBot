package filelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrLocked = errors.New("file lock is already held")

type Lock struct {
	file *os.File
	path string
}

func Acquire(path string) (*Lock, error) {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil, errors.New("lock path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock parent directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return AcquireFile(file)
}

// AcquireFile takes ownership of an already opened lock file, including on
// failure. Callers can use os.Root to enforce their filesystem boundary.
func AcquireFile(file *os.File) (*Lock, error) {
	if file == nil {
		return nil, errors.New("lock file is required")
	}
	if err := lockFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &Lock{file: file, path: file.Name()}, nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	var closeErr error
	if err := unlockFile(l.file); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("unlock file %s: %w", l.path, err))
	}
	if err := l.file.Close(); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("close lock file %s: %w", l.path, err))
	}
	l.file = nil
	return closeErr
}
