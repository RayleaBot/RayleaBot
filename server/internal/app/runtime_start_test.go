package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
)

func TestEnsureRuntimeStartedForEventStartsFirstEnabledInstalledPlugin(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	createPluginEntry(t, repoRoot, "examples/plugins/hello-node", "index.js")
	createPluginEntry(t, repoRoot, "examples/plugins/zzz-plugin", "main.py")

	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:     "aaa-invalid",
			Valid:        false,
			ManifestPath: "examples/plugins/aaa-invalid/info.json",
		},
		{
			PluginID:     "hello-node",
			Valid:        true,
			Runtime:      "nodejs",
			Entry:        "index.js",
			ManifestPath: "examples/plugins/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
		{
			PluginID:     "zzz-plugin",
			Valid:        true,
			Runtime:      "python",
			Entry:        "main.py",
			ManifestPath: "examples/plugins/zzz-plugin/info.json",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		},
	})
	manager := &fakeRuntimeStarter{
		snapshot: runtime.Snapshot{State: runtime.StateStopped},
	}

	snapshot, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.RuntimeConfig{},
		adapter.NormalizedEvent{BotID: "10001"},
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
}

func TestEnsureRuntimeStartedForEventSkipsWhenRuntimeIsAlreadyRunning(t *testing.T) {
	t.Parallel()

	manager := &fakeRuntimeStarter{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
	}
	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:     "hello-node",
			Valid:        true,
			Runtime:      "nodejs",
			Entry:        "index.js",
			ManifestPath: "examples/plugins/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		t.TempDir(),
		config.RuntimeConfig{},
		adapter.NormalizedEvent{BotID: "10001"},
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

func TestEnsureRuntimeStartedForEventRequiresBotID(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	createPluginEntry(t, repoRoot, "examples/plugins/hello-node", "index.js")

	manager := &fakeRuntimeStarter{
		snapshot: runtime.Snapshot{State: runtime.StateStopped},
	}
	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:     "hello-node",
			Valid:        true,
			Runtime:      "nodejs",
			Entry:        "index.js",
			ManifestPath: "examples/plugins/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "enabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.RuntimeConfig{},
		adapter.NormalizedEvent{},
	)
	if err == nil {
		t.Fatal("expected missing bot id to fail runtime startup")
	}
	if started {
		t.Fatal("runtime should not start without a bot id")
	}
}

func TestEnsureRuntimeStartedForEventSkipsDisabledPlugin(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	createPluginEntry(t, repoRoot, "examples/plugins/hello-node", "index.js")

	manager := &fakeRuntimeStarter{
		snapshot: runtime.Snapshot{State: runtime.StateStopped},
	}
	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:          "hello-node",
			Valid:             true,
			Runtime:           "nodejs",
			Entry:             "index.js",
			ManifestPath:      "examples/plugins/hello-node/info.json",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		},
	})

	_, started, err := ensureRuntimeStartedForEvent(
		context.Background(),
		manager,
		catalog,
		repoRoot,
		config.RuntimeConfig{},
		adapter.NormalizedEvent{BotID: "10001"},
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
	snapshot       runtime.Snapshot
	startCount     int
	startedSpec    runtime.Spec
	startedPayload runtime.InitPayload
	startErr       error
}

func (f *fakeRuntimeStarter) Snapshot() runtime.Snapshot {
	return f.snapshot
}

func (f *fakeRuntimeStarter) Start(_ context.Context, spec runtime.Spec, payload runtime.InitPayload) error {
	f.startCount++
	f.startedSpec = spec
	f.startedPayload = payload
	if f.startErr != nil {
		return f.startErr
	}

	f.snapshot = runtime.Snapshot{
		PluginID: spec.PluginID,
		State:    runtime.StateRunning,
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
