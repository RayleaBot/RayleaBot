package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

// WritePlatformDepsManifest writes a .deps manifest whose resource ids carry
// the current platform suffix, matching the ids the runtime bootstrap flow
// resolves for the host platform, plus the builtin render template fixtures.
func WritePlatformDepsManifest(t testing.TB, repoRoot string) {
	t.Helper()
	platform := deps.CurrentPlatform()
	chromiumID := "chromium-" + platform
	manifest := `{
  "manifest_version": 5,
  "resources": [
    {
      "id": "` + chromiumID + `",
      "kind": "chromium",
      "version": "152.0.7977.42",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "5093f03a401b5579da490d281aba80b687d92fe6fdfec47ee522920918d6e327",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    },
    {
      "id": "ffmpeg-` + platform + `",
      "kind": "ffmpeg",
      "version": "9.0.1",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/ffmpeg.zip", "kind": "upstream"}],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "zip",
      "entrypoints": {
        "ffmpeg": ["bin/ffmpeg"],
        "ffprobe": ["bin/ffprobe"]
      }
    }
  ]
}`
	path := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir deps manifest root: %v", err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
	WriteTestTemplate(t, repoRoot, "help.menu", 640)
	WriteTestTemplate(t, repoRoot, "status.panel", 540)
}

// WritePreparedRuntime marks a Chromium entrypoint as prepared in the repo-local .deps store.
func WritePreparedRuntime(t testing.TB, repoRoot, id, version string, segments ...string) {
	t.Helper()
	target := filepath.Join(append([]string{repoRoot, ".deps", "store", id, version}, segments...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir runtime target: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write runtime target: %v", err)
	}
}

// WaitTask polls the registry until the task reaches the wanted status or the
// deadline passes.
func WaitTask(t testing.TB, registry *tasks.Registry, taskID string, want tasks.Status) tasks.Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok && snapshot.Status == want {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}
	snapshot, _ := registry.Get(taskID)
	t.Fatalf("task %s did not reach %s: %#v", taskID, want, snapshot)
	return tasks.Snapshot{}
}
