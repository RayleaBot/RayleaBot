//go:build windows

package recovery

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRestoreRejectsWindowsTargetJunction(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	sentinel := filepath.Join(outside, "state.json")
	if err := os.WriteFile(sentinel, []byte("outside sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "data"), outside)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v %s", err, output)
	}
	if _, err := Restore(t.Context(), RestoreOptions{
		ConfigPath:  filepath.Join(root, "config", "user.yaml"),
		ArchivePath: createRestoreFixture(t, restoredDatabaseEntry),
	}); err == nil {
		t.Fatal("target directory junction accepted")
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside sentinel" {
		t.Fatalf("junction target changed: %q %v", got, err)
	}
}
