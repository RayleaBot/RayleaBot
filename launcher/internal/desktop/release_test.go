package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
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
func TestDevelopmentBuildOffersReleasePage(t *testing.T) {
	snapshot := NewReleaseFeed(t.TempDir()).GetSnapshot(true)
	if snapshot.Status != "disabled" || snapshot.ReleasePageURL == "" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}
