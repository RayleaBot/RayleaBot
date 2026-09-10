package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreCopyFailurePreservesExistingFile(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "state.json", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("new state")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archive := data.Bytes()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	offset, err := reader.File[0].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	archive[offset] ^= 1
	target := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(target, []byte("old state"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreFile(reader.File[0], target); err == nil {
		t.Fatal("corrupted archive entry was restored")
	}
	actual, err := os.ReadFile(target)
	if err != nil || string(actual) != "old state" {
		t.Fatalf("existing state lost: %q, %v", actual, err)
	}
}
