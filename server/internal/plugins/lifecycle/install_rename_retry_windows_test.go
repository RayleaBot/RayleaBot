//go:build windows

package lifecycle

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

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
