package filestore

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestServiceReadWriteDeleteAndList(t *testing.T) {
	t.Parallel()

	service := NewService(filepath.Join(t.TempDir(), "plugins"))
	limits := Limits{
		FileMaxBytes:  1024,
		TotalMaxBytes: 4096,
	}

	if err := service.Write("scope-cache", "cache/example.txt", []byte("hello"), limits); err != nil {
		t.Fatalf("Write text file: %v", err)
	}
	if err := service.Write("scope-cache", "cache/blob.bin", []byte{0xff, 0x00, 0x01}, limits); err != nil {
		t.Fatalf("Write binary file: %v", err)
	}

	textResult, err := service.Read("scope-cache", "cache/example.txt")
	if err != nil {
		t.Fatalf("Read text file: %v", err)
	}
	if !textResult.Exists || !textResult.IsText || string(textResult.Content) != "hello" {
		t.Fatalf("unexpected text read result: %+v", textResult)
	}

	binaryResult, err := service.Read("scope-cache", "cache/blob.bin")
	if err != nil {
		t.Fatalf("Read binary file: %v", err)
	}
	if !binaryResult.Exists || binaryResult.IsText {
		t.Fatalf("unexpected binary read result: %+v", binaryResult)
	}
	if got, want := base64.StdEncoding.EncodeToString(binaryResult.Content), "/wAB"; got != want {
		t.Fatalf("unexpected binary content: got %q want %q", got, want)
	}

	items, err := service.List("scope-cache", "cache")
	if err != nil {
		t.Fatalf("List files: %v", err)
	}
	if want := []string{"cache/blob.bin", "cache/example.txt"}; !reflect.DeepEqual(items, want) {
		t.Fatalf("unexpected listed files: got %#v want %#v", items, want)
	}

	deleted, err := service.Delete("scope-cache", "cache/example.txt")
	if err != nil {
		t.Fatalf("Delete file: %v", err)
	}
	if !deleted {
		t.Fatal("expected delete to report deleted=true")
	}

	missing, err := service.Read("scope-cache", "cache/example.txt")
	if err != nil {
		t.Fatalf("Read deleted file: %v", err)
	}
	if missing.Exists {
		t.Fatalf("expected deleted file to be missing: %+v", missing)
	}
}

func TestServiceRejectsPathEscapeAndSymlink(t *testing.T) {
	t.Parallel()

	service := NewService(filepath.Join(t.TempDir(), "plugins"))
	limits := Limits{FileMaxBytes: 1024, TotalMaxBytes: 4096}

	if err := service.Write("scope-cache", "../escape.txt", []byte("denied"), limits); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Write path escape error = %v, want ErrInvalidPath", err)
	}

	root, err := service.pluginRoot("scope-cache")
	if err != nil {
		t.Fatalf("pluginRoot: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}

	externalPath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(externalPath, []byte("outside"), 0o644); err != nil {
		t.Fatalf("Write external file: %v", err)
	}

	linkPath := filepath.Join(root, "linked.txt")
	if err := os.Symlink(externalPath, linkPath); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	if _, err := service.Read("scope-cache", "linked.txt"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Read symlink error = %v, want ErrInvalidPath", err)
	}
}

func TestServiceEnforcesWorkspaceQuota(t *testing.T) {
	t.Parallel()

	service := NewService(filepath.Join(t.TempDir(), "plugins"))
	limits := Limits{
		FileMaxBytes:  8,
		TotalMaxBytes: 10,
	}

	if err := service.Write("scope-cache", "a.txt", []byte("12345"), limits); err != nil {
		t.Fatalf("Write first file: %v", err)
	}

	if err := service.Write("scope-cache", "b.txt", []byte("123456"), limits); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Write quota error = %v, want ErrQuotaExceeded", err)
	}
	if err := service.Write("scope-cache", "c.txt", []byte("123456789"), limits); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Write file size error = %v, want ErrFileTooLarge", err)
	}
}
