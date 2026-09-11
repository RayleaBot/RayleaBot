package fsguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplacesContentAndLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil || string(content) != "new" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("unexpected directory entries: %v", entries)
	}
}

func TestWriteFileAtomicRequiresExistingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "state.json")
	if err := WriteFileAtomic(path, []byte("x"), 0o600); err == nil {
		t.Fatal("expected error for missing parent directory")
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("parent directory should not be created: %v", err)
	}
}
