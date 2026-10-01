package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReleaseCheckProvidesManualReleaseLink(t *testing.T) {
	result, err := parseReleaseCheck(`{"status":"update_available","current_version":"1.0.0","available_version":"1.1.0","release_page_url":"https://github.com/RayleaBot/RayleaBot/releases/tag/v1.1.0"}`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotFromCheck(result)
	if !snapshot.UpdateAvailable || !snapshot.CanCheck || snapshot.ReleasePageURL != result.ReleasePageURL {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}
func TestReleaseCheckRejectsInvalidResponse(t *testing.T) {
	for _, payload := range []string{`{}`, `{"status":"installing"}`, `{"status":"update_available","current_version":"1.0.0","available_version":"1.1.0","release_page_url":"javascript:alert(1)"}`} {
		if _, err := parseReleaseCheck(payload); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
}
func TestBuildInfoSupportsAllDesktopPlatformsWithoutUpdater(t *testing.T) {
	for _, platform := range []struct{ goos, goarch, id string }{{"windows", "amd64", ArtifactWindowsX64Full}, {"linux", "amd64", ArtifactLinuxX64Full}, {"darwin", "arm64", ArtifactMacOSARM64Full}} {
		t.Run(platform.goos, func(t *testing.T) {
			root := t.TempDir()
			data, _ := json.Marshal(buildInfo{Version: "1.2.3", ArtifactID: platform.id})
			if err := os.WriteFile(filepath.Join(root, "build_info.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			info, err := readBuildInfoForPlatform(root, platform.goos, platform.goarch)
			if err != nil || info.Version != "1.2.3" {
				t.Fatalf("info=%#v err=%v", info, err)
			}
		})
	}
}

type releaseRecordingHost struct {
	testServiceHost
	releases []ReleaseCheckSnapshot
}

func (h *releaseRecordingHost) Emit(_ string, data any) {
	if snapshot, ok := data.(LauncherSnapshot); ok {
		h.releases = append(h.releases, snapshot.Launcher.ReleaseCheck)
	}
}

func TestReleaseCheckShowsInstalledVersionBeforeTheCheckFinishes(t *testing.T) {
	artifactID := launcherArtifactID(runtime.GOOS, runtime.GOARCH)
	if artifactID == "" {
		t.Skip("no release artifact for this platform")
	}
	root := t.TempDir()
	data, _ := json.Marshal(buildInfo{Version: "1.2.3", ArtifactID: artifactID})
	writeTestFile(t, filepath.Join(root, "build_info.json"), string(data))
	host := &releaseRecordingHost{}

	NewCoordinator(root, "", 0, host).refreshRelease(true)

	if len(host.releases) == 0 || host.releases[0].Status != ReleaseChecking || host.releases[0].CurrentVersion != "1.2.3" {
		t.Fatalf("release states = %#v, want the installed version while checking", host.releases)
	}
}

func TestDevelopmentBuildOffersReleasePage(t *testing.T) {
	snapshot := NewReleaseFeed(t.TempDir()).GetSnapshot(true)
	if snapshot.Status != "disabled" || snapshot.ReleasePageURL == "" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}
