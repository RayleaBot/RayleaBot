//go:build windows

package releaseupdate

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHelperProcessUpdateSleep(t *testing.T) {
	if os.Getenv("RAYLEABOT_UPDATE_SLEEP_HELPER") != "1" {
		return
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

// The Launcher runs update apply while its own executable and the helper binary
// are running; Windows allows renaming such files but not overwriting them.
func TestApplyReplacesRunningExecutable(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	running := filepath.Join(root, "raylea-server.exe")
	if err := os.WriteFile(running, binary, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(running, "-test.run=^TestHelperProcessUpdateSleep$")
	command.Env = append(os.Environ(), "RAYLEABOT_UPDATE_SLEEP_HELPER=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})

	checker, _ := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	if _, err := checker.Apply(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertTree(t, root, map[string]string{"raylea-server.exe": "new server"})
	if InstalledVersion(root) != "1.0.0" {
		t.Fatalf("installed version = %s", InstalledVersion(root))
	}
}
