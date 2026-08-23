package filelock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const childLockPathEnv = "RAYLEABOT_FILELOCK_CHILD_PATH"

func TestAcquirePreventsOverlappingLocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer first.Close()

	if _, err := Acquire(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire error = %v, want ErrLocked", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first lock: %v", err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("reacquire released lock: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second lock: %v", err)
	}
}

func TestAcquirePreventsCrossProcessLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.lock")
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire parent lock: %v", err)
	}
	defer lock.Close()

	command := exec.Command(os.Args[0], "-test.run=^TestAcquireInChildProcess$")
	command.Env = append(os.Environ(), childLockPathEnv+"="+path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("child process did not observe parent lock: %v\n%s", err, output)
	}
}

func TestAcquireInChildProcess(t *testing.T) {
	path := os.Getenv(childLockPathEnv)
	if path == "" {
		t.Skip("helper test only runs in a child process")
	}
	lock, err := Acquire(path)
	if lock != nil {
		_ = lock.Close()
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("child acquire error = %v, want ErrLocked", err)
	}
}
