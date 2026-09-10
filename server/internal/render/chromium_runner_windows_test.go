//go:build windows

package render

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestChromiumRunnerReportsOwnedProfileRemovalFailure(t *testing.T) {
	profile := t.TempDir()
	file := filepath.Join(profile, "locked")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	nativePath, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(nativePath, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(handle) })
	runner := NewChromiumRunner(ChromiumOptions{})
	runner.profileDir = profile
	failure := runner.Close()
	if failure == nil {
		t.Fatal("profile removal failure was ignored")
	}
	if err := runner.Close(); !errors.Is(err, failure) {
		t.Fatalf("repeated close lost failure: %v", err)
	}
	if runner.profileDir != profile {
		t.Fatal("failed cleanup lost the owned profile path")
	}
}
