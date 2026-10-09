//go:build windows

package lifecycle

import (
	"archive/zip"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestExtractZipRejectsWindowsDirectoryJunction(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	extractRoot := filepath.Join(root, "unzipped")
	if err := os.Mkdir(extractRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(extractRoot, "plugin"), outside)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v %s", err, output)
	}
	archive := filepath.Join(t.TempDir(), "plugin.zip")
	writeZipEntries(t, archive, []zipTestEntry{{header: zip.FileHeader{Name: "plugin/payload.txt", Method: zip.Store}, content: "payload"}})
	if _, err := extractZipSource(t.Context(), archive, root); err == nil {
		t.Fatal("accepted junction outside extraction root")
	}
	if _, err := os.Stat(filepath.Join(outside, "payload.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrote outside root: %v", err)
	}
}
