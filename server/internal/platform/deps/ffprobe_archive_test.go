package deps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFFprobeArchivePreparationIsCompleteAndAtomic(t *testing.T) {
	for _, outcome := range []string{"success", "download failure", "digest mismatch", "missing entrypoint", "directory collision"} {
		t.Run(outcome, func(t *testing.T) {
			root := t.TempDir()
			mpegZip, probeZip := filepath.Join(t.TempDir(), "ffmpeg.zip"), filepath.Join(t.TempDir(), "ffprobe.zip")
			mainFiles := map[string]string{"ffmpeg": "fixture ffmpeg"}
			if outcome == "directory collision" {
				mainFiles["ffprobe/unrelated"] = "fixture"
			}
			writeZipArchive(t, mpegZip, mainFiles)
			probeFiles := map[string]string{"ffprobe": "fixture ffprobe"}
			if outcome == "missing entrypoint" {
				probeFiles = map[string]string{"other": "fixture"}
			}
			writeZipArchive(t, probeZip, probeFiles)
			mpegBytes, err := os.ReadFile(mpegZip)
			if err != nil {
				t.Fatal(err)
			}
			probeBytes, err := os.ReadFile(probeZip)
			if err != nil {
				t.Fatal(err)
			}
			resource := Resource{
				ID: "ffmpeg-test", Kind: "ffmpeg", Version: "1", Platform: CurrentPlatform(),
				Sources: []ResourceSource{{URL: "https://example.invalid/ffmpeg.zip", Kind: "upstream"}},
				SHA256:  sha256Hex(mpegBytes), ArchiveFormat: "zip",
				Entrypoints:    map[string][]string{"ffmpeg": {"ffmpeg"}, "ffprobe": {"ffprobe/ffprobe"}},
				FFprobeArchive: &ResourceArchive{Sources: []ResourceSource{{URL: "https://example.invalid/ffprobe.zip", Kind: "upstream"}}, SHA256: sha256Hex(probeBytes), ArchiveFormat: "zip"},
			}
			writeDepsManifest(t, root, ManifestVersion, resource)
			manager := NewManager(root)
			manager.downloadFile = func(_ context.Context, source, destination string) error {
				data := mpegBytes
				if source == resource.FFprobeArchive.Sources[0].URL {
					if outcome == "download failure" {
						return errors.New("fixture failure")
					}
					data = probeBytes
					if outcome == "digest mismatch" {
						data = []byte("corrupt")
					}
				}
				return os.WriteFile(destination, data, 0o600)
			}
			report, err := manager.PrepareWithReport(t.Context(), "ffmpeg")
			if outcome != "success" {
				if err == nil {
					t.Fatal("incomplete preparation succeeded")
				}
				if _, err := os.Stat(StoreRoot(root, &resource)); !os.IsNotExist(err) {
					t.Fatalf("partial store was activated: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Entrypoints) != 2 {
				t.Fatalf("missing entrypoints: %#v", report.Entrypoints)
			}
			inspection, err := manager.Inspect("ffmpeg")
			if err != nil || !inspection.CachedArchivePresent || !inspection.PreparedStorePresent {
				t.Fatalf("inspection: %#v %v", inspection, err)
			}
			if err := os.Remove(ffprobeArchivePath(root, resource.ffprobeResource())); err != nil {
				t.Fatal(err)
			}
			inspection, err = manager.Inspect("ffmpeg")
			if err != nil || inspection.CachedArchivePresent {
				t.Fatalf("incomplete cache reported ready: %#v %v", inspection, err)
			}
		})
	}
}
