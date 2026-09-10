package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireFileOwnsOpenedHandleOnLockFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	lock, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireFile(file); !errors.Is(err, ErrLocked) {
		_ = file.Close()
		t.Fatalf("second lock = %v", err)
	}
	if err := file.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed lock kept file open: %v", err)
	}
}
