package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
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
  "manifest_version": 4,
  "resources": [
    {
      "id": "` + chromiumID + `",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
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
