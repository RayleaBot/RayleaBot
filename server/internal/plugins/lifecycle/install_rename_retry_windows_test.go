//go:build windows

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestInstallRenameRetriesRealWindowsSharingViolation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "candidate.exe")
	target := filepath.Join(filepath.Dir(source), "installed.exe")
	if err := os.WriteFile(source, []byte("offline executable fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(source)
	if err != nil {
		t.Fatal(err)
	}
	// An open executable/virus scanner may temporarily deny FILE_SHARE_DELETE.
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if handle != windows.InvalidHandle {
			_ = windows.CloseHandle(handle)
		}
	})
	var attempts, waits int
	service := &InstallService{deps: installerDeps{
		rename: func(from, to string) error {
			attempts++
			return os.Rename(from, to)
		},
		retryRename: isRetryableInstallRenameError,
		waitRename: func(context.Context) error {
			waits++
			err := windows.CloseHandle(handle)
			handle = windows.InvalidHandle
			return err
		},
	}}
	if err := service.renameInstallPath(t.Context(), source, target); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || waits != 1 {
		t.Fatalf("attempts=%d, waits=%d: expected one real sharing violation", attempts, waits)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate still exists: %v", err)
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "offline executable fixture" {
		t.Fatalf("installed contents: %q, %v", content, err)
	}
}

func TestIsRetryableInstallRenameError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "access denied", err: windows.ERROR_ACCESS_DENIED, want: true},
		{name: "sharing violation", err: windows.ERROR_SHARING_VIOLATION, want: true},
		{name: "lock violation", err: windows.ERROR_LOCK_VIOLATION, want: true},
		{name: "wrapped retryable error", err: fmt.Errorf("rename: %w", windows.ERROR_SHARING_VIOLATION), want: true},
		{name: "file not found", err: windows.ERROR_FILE_NOT_FOUND, want: false},
		{name: "generic error", err: errors.New("rename failed"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableInstallRenameError(test.err); got != test.want {
				t.Fatalf("isRetryableInstallRenameError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
