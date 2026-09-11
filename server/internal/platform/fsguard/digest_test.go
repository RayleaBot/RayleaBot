package fsguard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryDigestFormatAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"dir/b.txt": "B", "a.txt": "A"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := SHA256Directory(t.Context(), root)
	// SHA256 of a.txt\0A\0dir/b.txt\0B\0, independent of creation order.
	if err != nil || digest != "da0c53c8de0e68cf66fe64c6b69a477272d68c12af886e3b90957fcf69f3d63b" {
		t.Fatalf("directory digest = %q, %v", digest, err)
	}
	if err := os.Rename(filepath.Join(root, "a.txt"), filepath.Join(root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	changed, err := SHA256Directory(t.Context(), root)
	if err != nil || changed == digest {
		t.Fatalf("file names must contribute: %q, %v", changed, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, path := range []string{root, t.TempDir()} {
		if _, err := SHA256Directory(ctx, path); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled digest: %v", err)
		}
	}
}

func TestFileDigestBoundAndCancellation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(file, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := SHA256File(t.Context(), file, 3)
	if err != nil || digest != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("file digest = %q, %v", digest, err)
	}
	if _, err := SHA256File(t.Context(), file, 2); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("size limit: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := SHA256File(ctx, file, 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled digest: %v", err)
	}
	if _, err := SHA256File(t.Context(), filepath.Dir(file), 3); err == nil {
		t.Fatal("directory accepted as a file")
	}
}
