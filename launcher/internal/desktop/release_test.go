package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTrustedCheckRejectsUnsafeReleaseSurface(t *testing.T) {
	base := `{"status":"update_available","current_version":"1.0.0","available_version":"1.1.0","update_mode":"automatic","automatic_install_supported":true,"release_page_url":"https://github.com/RayleaBot/RayleaBot/releases/tag/v1.1.0","artifact":{"artifact_id":"windows-x64-full","file_name":"RayleaBot-v1.1.0.zip","archive_size_bytes":1024,"update_mode":"automatic"},"artifact_path":"C:/cache/update.zip"}`
	if _, err := parseWindowsUpdaterCheck(base, true); err != nil {
		t.Fatalf("parseWindowsUpdaterCheck(valid) error = %v", err)
	}
	unsafe := `{"status":"update_available","current_version":"1.0.0","available_version":"1.1.0","update_mode":"automatic","automatic_install_supported":true,"release_page_url":"https://user:pass@example.com/release","artifact":{"artifact_id":"windows-x64-full","file_name":"../update.zip","archive_size_bytes":1024,"update_mode":"automatic"},"artifact_path":"C:/cache/update.zip"}`
	if _, err := parseWindowsUpdaterCheck(unsafe, true); err == nil {
		t.Fatal("parseWindowsUpdaterCheck() accepted unsafe release metadata")
	}
}

func TestReadBuildInfoAcceptsMatchingLauncherArtifact(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		goarch     string
		artifactID string
	}{
		{name: "windows-x64", goos: "windows", goarch: "amd64", artifactID: "windows-x64-full"},
		{name: "linux-x64", goos: "linux", goarch: "amd64", artifactID: "linux-x64-full"},
		{name: "macos-arm64", goos: "darwin", goarch: "arm64", artifactID: "macos-arm64-full"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "build_info.json"), fmt.Sprintf(
				`{"version":"1.2.3","artifact_id":%q,"update_protocol_version":2}`,
				test.artifactID,
			))
			info, err := readBuildInfoForPlatform(root, test.goos, test.goarch)
			if err != nil || info.ArtifactID != test.artifactID {
				t.Fatalf("readBuildInfoForPlatform() = %#v, %v", info, err)
			}
		})
	}
}

func TestGuidedPlatformsExposePackagedVersionAndReleasePage(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		goarch     string
		artifactID string
	}{
		{name: "linux-x64", goos: "linux", goarch: "amd64", artifactID: "linux-x64-full"},
		{name: "macos-arm64", goos: "darwin", goarch: "arm64", artifactID: "macos-arm64-full"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "build_info.json"), fmt.Sprintf(
				`{"version":"1.2.3","artifact_id":%q,"update_protocol_version":2}`,
				test.artifactID,
			))

			snapshot := NewReleaseFeed(root).getSnapshot(true, test.goos, test.goarch)
			if snapshot.Status != "disabled" || snapshot.ErrorCode != "" || snapshot.CurrentVersion != "1.2.3" {
				t.Fatalf("guided snapshot = %#v", snapshot)
			}
			if snapshot.ReleasePageURL != repositoryURL+"/releases/latest" || snapshot.CanCheck || snapshot.CanDownload || snapshot.CanInstall {
				t.Fatalf("guided release surface = %#v", snapshot)
			}
		})
	}
}

func TestBuildInfoRejectsArtifactForAnotherPlatform(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "build_info.json"), `{"version":"1.2.3","artifact_id":"linux-x64-server","update_protocol_version":2}`)
	if _, err := readBuildInfoForPlatform(root, "linux", "amd64"); err == nil {
		t.Fatal("readBuildInfoForPlatform() accepted the server-only artifact for Launcher")
	}

	writeTestFile(t, filepath.Join(root, "build_info.json"), `{"version":"1.2.3","artifact_id":"linux-x64-full","update_protocol_version":2}`)
	if _, err := readBuildInfoForPlatform(root, "darwin", "arm64"); err == nil {
		t.Fatal("readBuildInfoForPlatform() accepted a build for another platform")
	}
}

func TestTransactionPathsRemainWithinControlledLocations(t *testing.T) {
	root := t.TempDir()
	transaction := root + "-parent/.rayleabot-update-test"
	if transactionSibling(root, transaction) {
		t.Fatal("transactionSibling() accepted a different parent")
	}
	if pathInside(root, root) || pathInside(root, root+"-sibling/file") {
		t.Fatal("pathInside() accepted root or sibling")
	}
}

func TestSanitizeReleaseDetailRedactsKnownUpdatePaths(t *testing.T) {
	installRoot := filepath.Join(t.TempDir(), "RayleaBot")
	updaterPath := filepath.Join(installRoot, "raylea-updater.exe")
	transactionRoot := filepath.Join(filepath.Dir(installRoot), ".rayleabot-update-test")
	detail := fmt.Sprintf(
		"open %s failed; install root %s; transaction input %s",
		filepath.ToSlash(updaterPath),
		filepath.ToSlash(installRoot),
		filepath.Join(transactionRoot, "release_manifest.v2.json"),
	)

	sanitized := sanitizeReleaseDetail(detail, releaseFailureContext{
		installRoot: installRoot, updaterPath: updaterPath, transactionRoot: transactionRoot,
	})
	for _, sensitivePath := range []string{installRoot, filepath.ToSlash(installRoot), transactionRoot, filepath.ToSlash(transactionRoot)} {
		if strings.Contains(strings.ToLower(sanitized), strings.ToLower(sensitivePath)) {
			t.Fatalf("sanitized detail still contains %q: %q", sensitivePath, sanitized)
		}
	}
	for _, replacement := range []string{"raylea-updater.exe", "<安装目录>", "<更新事务目录>"} {
		if !strings.Contains(sanitized, replacement) {
			t.Fatalf("sanitized detail = %q, want %q", sanitized, replacement)
		}
	}
}

func TestInstallFailurePreservesRetryActionsAndReleaseContext(t *testing.T) {
	progress := float64(1)
	downloadedBytes := int64(1024)
	feed := NewReleaseFeed(t.TempDir())
	feed.downloaded = &trustedCheck{AutomaticInstallSupported: true}
	feed.cached = ReleaseCheckSnapshot{
		Status:           "ready_to_install",
		CurrentVersion:   "1.0.0",
		LatestVersion:    "1.1.0",
		ReleasePageURL:   "https://github.com/RayleaBot/RayleaBot/releases/tag/v1.1.0",
		UpdateAvailable:  true,
		DownloadProgress: &progress,
		DownloadedBytes:  &downloadedBytes,
		TotalBytes:       &downloadedBytes,
		ArtifactFileName: "RayleaBot-v1.1.0.zip",
		CanCheck:         true,
		CanInstall:       true,
	}

	failed := feed.installFailure(
		"failed",
		"release.install_failed",
		"更新安装失败。",
		"更新助手启动失败",
		"1.0.0",
	)

	if !failed.CanCheck || !failed.CanInstall {
		t.Fatalf("failed install actions = check:%t install:%t", failed.CanCheck, failed.CanInstall)
	}
	if failed.LatestVersion != "1.1.0" || failed.ReleasePageURL == "" || failed.ArtifactFileName == "" || !failed.UpdateAvailable {
		t.Fatalf("failed install lost release context: %#v", failed)
	}
	if !releaseReadyForInstall(failed) {
		t.Fatalf("failed install cannot be retried: %#v", failed)
	}
}
