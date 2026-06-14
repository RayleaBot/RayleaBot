package startup

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/runtime/spec"
)

func TestEnsureRuntimeStartedForEventStartsFirstEnabledInstalledPlugin(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeManagedRuntimeFixtures(t, repoRoot)
	createPluginEntry(t, repoRoot, "plugins/installed/hello-node", "index.js")
	createPluginEntry(t, repoRoot, "plugins/installed/zzz-plugin", "main.py")

	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:     "aaa-invalid",
			Valid:        false,
			ManifestPath: "plugins/installed/aaa-invalid/info.json",
		},
		{
			PluginID:          "hello-node",
			Valid:             true,
			Runtime:           "nodejs",
			Entry:             "index.js",
			ManifestPath:      "plugins/installed/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
		{
			PluginID:          "zzz-plugin",
			Valid:             true,
			Runtime:           "python",
			Entry:             "main.py",
			ManifestPath:      "plugins/installed/zzz-plugin/info.json",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		},
	})
	manager := &fakeRuntimeStarter{
		snapshot: runtimemanager.Snapshot{State: runtimemanager.StateStopped},
	}

	snapshot, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.Config{
			Admin: config.AdminConfig{
				SuperAdmins: []string{"10001", "10002", "10001", " "},
			},
			Command: &config.CommandConfig{Prefixes: []string{"!", "/"}},
		},
		adapterintake.NormalizedEvent{BotID: "10001"},
	)
	if err != nil {
		t.Fatalf("ensure runtime started: %v", err)
	}
	if !started {
		t.Fatal("expected runtime to start for the first enabled installed plugin")
	}
	if snapshot.PluginID != "hello-node" {
		t.Fatalf("unexpected startup plugin: got %q want %q", snapshot.PluginID, "hello-node")
	}
	if manager.startCount != 1 {
		t.Fatalf("unexpected start count: got %d want 1", manager.startCount)
	}
	if manager.startedSpec.PluginID != "hello-node" {
		t.Fatalf("unexpected started plugin: got %q want %q", manager.startedSpec.PluginID, "hello-node")
	}
	if manager.startedPayload.Bot.ID != "10001" {
		t.Fatalf("unexpected bot id: got %q want %q", manager.startedPayload.Bot.ID, "10001")
	}
	if len(manager.startedPayload.CommandPrefixes) != 2 || manager.startedPayload.CommandPrefixes[0] != "!" || manager.startedPayload.CommandPrefixes[1] != "/" {
		t.Fatalf("unexpected command prefixes: %#v", manager.startedPayload.CommandPrefixes)
	}
	if len(manager.startedPayload.SuperAdmins) != 2 || manager.startedPayload.SuperAdmins[0] != "10001" || manager.startedPayload.SuperAdmins[1] != "10002" {
		t.Fatalf("unexpected super admins: %#v", manager.startedPayload.SuperAdmins)
	}
}

func TestEnsureRuntimeStartedForEventSkipsWhenRuntimeIsAlreadyRunning(t *testing.T) {
	t.Parallel()

	manager := &fakeRuntimeStarter{
		snapshot: runtimemanager.Snapshot{State: runtimemanager.StateRunning},
	}
	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "hello-node",
			Valid:             true,
			Runtime:           "nodejs",
			Entry:             "index.js",
			ManifestPath:      "plugins/installed/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		t.TempDir(),
		config.Config{},
		adapterintake.NormalizedEvent{BotID: "10001"},
	)
	if err != nil {
		t.Fatalf("ensure runtime started: %v", err)
	}
	if started {
		t.Fatal("runtime should not restart while already running")
	}
	if manager.startCount != 0 {
		t.Fatalf("unexpected start count: got %d want 0", manager.startCount)
	}
}

func TestEnsureRuntimeStartedForEventAllowsMissingBotID(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeManagedRuntimeFixtures(t, repoRoot)
	createPluginEntry(t, repoRoot, "plugins/installed/hello-node", "index.js")

	manager := &fakeRuntimeStarter{
		snapshot: runtimemanager.Snapshot{State: runtimemanager.StateStopped},
	}
	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "hello-node",
			Valid:             true,
			Runtime:           "nodejs",
			Entry:             "index.js",
			ManifestPath:      "plugins/installed/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.Config{},
		adapterintake.NormalizedEvent{},
	)
	if err != nil {
		t.Fatalf("ensure runtime started without bot id: %v", err)
	}
	if !started {
		t.Fatal("runtime should start without a bot id")
	}
	if manager.startedPayload.Bot.ID != "" {
		t.Fatalf("unexpected bot id: got %q want empty", manager.startedPayload.Bot.ID)
	}
}

func TestEnsureRuntimeStartedForEventSkipsDisabledPlugin(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeManagedRuntimeFixtures(t, repoRoot)
	createPluginEntry(t, repoRoot, "plugins/installed/hello-node", "index.js")

	manager := &fakeRuntimeStarter{
		snapshot: runtimemanager.Snapshot{State: runtimemanager.StateStopped},
	}
	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "hello-node",
			Valid:             true,
			Runtime:           "nodejs",
			Entry:             "index.js",
			ManifestPath:      "plugins/installed/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.Config{},
		adapterintake.NormalizedEvent{BotID: "10001"},
	)
	if err != nil {
		t.Fatalf("ensure runtime started: %v", err)
	}
	if started {
		t.Fatal("runtime should not start for a disabled plugin")
	}
	if manager.startCount != 0 {
		t.Fatalf("unexpected start count: got %d want 0", manager.startCount)
	}
}

type fakeRuntimeStarter struct {
	snapshot       runtimemanager.Snapshot
	startCount     int
	startedSpec    runtimespec.Spec
	startedPayload runtimespec.InitPayload
	startErr       error
}

func (f *fakeRuntimeStarter) Snapshot() runtimemanager.Snapshot {
	return f.snapshot
}

func (f *fakeRuntimeStarter) Start(_ context.Context, spec runtimespec.Spec, payload runtimespec.InitPayload) error {
	f.startCount++
	f.startedSpec = spec
	f.startedPayload = payload
	if f.startErr != nil {
		return f.startErr
	}

	f.snapshot = runtimemanager.Snapshot{
		PluginID: spec.PluginID,
		State:    runtimemanager.StateRunning,
	}
	return nil
}

func createPluginEntry(t *testing.T, repoRoot string, relativeDir string, entryName string) {
	t.Helper()

	pluginDir := filepath.Join(repoRoot, filepath.FromSlash(relativeDir))
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "info.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, entryName), []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
}

func writeManagedRuntimeFixtures(t *testing.T, repoRoot string) {
	t.Helper()

	platform := testManifestPlatform()
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir deps root: %v", err)
	}
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/install/bin/python3"]
      }
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/node.tar.xz",
          "kind": "upstream"
        }
      ],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "tar.xz",
      "entrypoints": {
        "node": ["node/bin/node"],
        "npm": ["node/bin/npm"]
      }
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "python3"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "pip3"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "node"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "npm"))
}

func writeManagedRuntimeEntry(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir managed runtime dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("runtime"), 0o755); err != nil {
		t.Fatalf("write managed runtime entry: %v", err)
	}
}

func testManifestPlatform() string {
	switch goruntime.GOOS {
	case "windows":
		if goruntime.GOARCH == "amd64" {
			return "windows-x64"
		}
		return "windows-" + goruntime.GOARCH
	case "darwin":
		if goruntime.GOARCH == "amd64" {
			return "macos-x64"
		}
		return "macos-" + goruntime.GOARCH
	default:
		if goruntime.GOARCH == "amd64" {
			return goruntime.GOOS + "-x64"
		}
		return goruntime.GOOS + "-" + goruntime.GOARCH
	}
}
