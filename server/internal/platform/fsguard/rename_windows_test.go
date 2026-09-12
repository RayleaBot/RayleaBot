//go:build windows

package fsguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRenameRetriesDirectoryHeldByWindows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, target := filepath.Join(root, "candidate"), filepath.Join(root, "installed")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "ffmpeg.exe")
	if err := os.WriteFile(file, []byte("native executable fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(release)
	waits := 0
	err = RenameWithRetry(t.Context(), source, target, RenameRetryOptions{
		Attempts: 3,
		Wait: func(context.Context) error {
			waits++
			release()
			return nil
		},
	})
	if err != nil || waits != 1 {
		t.Fatalf("rename with held file = %v, waits = %d", err, waits)
	}
	if content, err := os.ReadFile(filepath.Join(target, "ffmpeg.exe")); err != nil || string(content) != "native executable fixture" {
		t.Fatalf("activated executable: %q, %v", content, err)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging directory still exists: %v", err)
	}
}

func TestIsRetryableRenameError(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"access denied", windows.ERROR_ACCESS_DENIED, true},
		{"sharing violation", windows.ERROR_SHARING_VIOLATION, true},
		{"lock violation", windows.ERROR_LOCK_VIOLATION, true},
		{"wrapped sharing error", fmt.Errorf("rename: %w", windows.ERROR_SHARING_VIOLATION), true},
		{"file not found", windows.ERROR_FILE_NOT_FOUND, false},
		{"generic error", errors.New("rename failed"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableRenameError(tt.err); got != tt.want {
				t.Fatalf("retryable(%v) = %t", tt.err, got)
			}
		})
	}
}
