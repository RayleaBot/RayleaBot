package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLauncherExecutableFollowsReleaseLayout(t *testing.T) {
	root := filepath.Join("install", "root")
	for goos, want := range map[string]string{
		"windows": filepath.Join(root, "RayleaLauncher.exe"),
		"linux":   filepath.Join(root, "RayleaLauncher"),
		"darwin":  filepath.Join(root, "RayleaLauncher.app", "Contents", "MacOS", "RayleaLauncher"),
	} {
		if got := launcherExecutable(root, goos); got != want {
			t.Fatalf("launcherExecutable(%s) = %q, want %q", goos, got, want)
		}
	}
}

func TestWaitForProcessExitReturnsOnceProcessEnds(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^$")
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	WaitForProcessExit(command.ProcessState.Pid())
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("waited %s for an exited process", elapsed)
	}
}

func TestApplyUpdateDownloadFailureKeepsServiceAndOffersRetry(t *testing.T) {
	coordinator := NewCoordinator(t.TempDir(), "", 0, &testServiceHost{})
	coordinator.publishRelease(ReleaseCheckSnapshot{Status: ReleaseUpdateAvailable, CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateAvailable: true, CanCheck: true})
	if coordinator.ApplyUpdate() {
		t.Fatal("update reported success without a server binary")
	}
	release := coordinator.Snapshot().Launcher.ReleaseCheck
	if release.Status != ReleaseFailed || release.ErrorCode != "launcher.update_download_failed" || !release.UpdateAvailable || !release.CanCheck {
		t.Fatalf("release = %#v", release)
	}
	if coordinator.process.IsRunning() {
		t.Fatal("download failure touched the service process")
	}
}

func TestApplyUpdateIgnoresRequestsWithoutAvailableUpdate(t *testing.T) {
	coordinator := NewCoordinator(t.TempDir(), "", 0, &testServiceHost{})
	if coordinator.ApplyUpdate() {
		t.Fatal("update started without an available release")
	}
	if status := coordinator.Snapshot().Launcher.ReleaseCheck.Status; status == ReleaseUpdating || status == ReleaseFailed {
		t.Fatalf("release status changed to %s", status)
	}
}
