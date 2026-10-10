package fsguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryComparisonReadsContentAndNames(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(root, name string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want bool) {
		t.Helper()
		equal, err := EqualDirectoryContents(t.Context(), left, right)
		if err != nil || equal != want {
			t.Fatalf("equal=%v err=%v want=%v", equal, err, want)
		}
	}
	check(true)
	for index := range 32 {
		name := fmt.Sprintf("nested/%02d", index)
		// Exercise empty, exact-buffer and partial final reads across workers.
		data := bytes.Repeat([]byte{byte(index)}, index*8192)
		write(left, name, data)
		write(right, name, data)
	}
	check(true)
	file := filepath.Join(right, "nested", "31")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	write(right, "nested/31", bytes.Repeat([]byte{'x'}, int(info.Size())))
	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	check(false)
	write(right, "nested/31", bytes.Repeat([]byte{31}, int(info.Size())))
	check(true)
	write(right, "extra", []byte("new"))
	check(false)
	if err := os.Remove(filepath.Join(right, "extra")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	check(false)
	write(right, "nested/31", []byte("short"))
	check(false)
}

func TestDirectoryComparisonRejectsMissingRootsLinksAndCancellation(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	if _, err := EqualDirectoryContents(t.Context(), left, filepath.Join(right, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := EqualDirectoryContents(ctx, left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	file := filepath.Join(left, "file")
	if err := os.WriteFile(file, []byte("same"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(right, "file")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if equal, err := EqualDirectoryContents(t.Context(), left, right); err == nil || equal {
		t.Fatalf("accepted link: %v %v", equal, err)
	}
}
