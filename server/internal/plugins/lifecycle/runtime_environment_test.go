package lifecycle

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestRefreshManagedRuntimeEnvironment(t *testing.T) {
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	t.Setenv("RAYLEABOT_FFMPEG_PATH", "")
	t.Setenv("RAYLEABOT_FFPROBE_PATH", "")
	for _, fail := range []bool{false, true} {
		name := "prepared_paths_reach_running_plugins"
		if fail {
			name = "reload_failure_preserves_old_process"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testutil.WritePlatformDepsManifest(t, root)
			makePlugin := func(id string) plugins.Snapshot {
				dir := writeInstallSourcePlugin(t, filepath.Join(root, "plugins", id), id)
				if err := os.WriteFile(filepath.Join(dir, ".crash-cycle-probe"), []byte("runtime-environment"), 0o600); err != nil {
					t.Fatal(err)
				}
				return plugins.Snapshot{PluginID: id, Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "stopped", ManifestPath: filepath.Join(dir, "info.json"), PackageRootPath: dir}
			}
			stale, current := makePlugin("stale"), makePlugin("current")
			disabled, stopped, recovery := makePlugin("disabled"), makePlugin("stopped"), makePlugin("recovery")
			disabled.DesiredState = "disabled"
			recovery.RuntimeState = "dead_letter"
			cat := catalog.New([]plugins.Snapshot{stale, current, disabled, stopped, recovery})
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			runtimes := pluginruntime.NewRegistry(logger, pluginruntime.Options{})
			registry := tasks.NewRegistry()
			controller := newTestController(t, Deps{RepoRoot: root, PluginDataRoot: filepath.Join(root, "data"), Logger: logger, Plugins: cat, Runtimes: runtimes, Tasks: registry,
				CurrentConfig: func() config.Config {
					return config.Config{Scheduler: config.SchedulerConfig{Timezone: "UTC"}, Runtime: config.RuntimeConfig{PluginInitTimeoutSeconds: 10, ShutdownGraceSeconds: 2}}
				}})
			if err := controller.StartInstalled(t.Context(), stale.PluginID); err != nil {
				t.Fatal(err)
			}
			old, _ := runtimes.Get(stale.PluginID)
			readTools := func() map[string]string {
				t.Helper()
				body, err := os.ReadFile(filepath.Join(root, "data", stale.PluginID, "media-tools.json"))
				if err != nil {
					t.Fatal(err)
				}
				var tools map[string]string
				if err := json.Unmarshal(body, &tools); err != nil {
					t.Fatal(err)
				}
				return tools
			}
			if paths := readTools(); paths["ffmpeg"] != "" || paths["ffprobe"] != "" {
				t.Fatalf("tools unexpectedly prepared at startup: %#v", paths)
			}
			if err := controller.RefreshManagedRuntimeEnvironment(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(registry.List()) != 0 {
				t.Fatal("missing resources triggered reload")
			}

			for _, tool := range []string{"ffmpeg", "ffprobe"} {
				testutil.WriteTestRuntimeEntry(t, root, "ffmpeg-"+deps.CurrentPlatform(), "9.0.1", "bin", tool)
			}
			if err := controller.StartInstalled(t.Context(), current.PluginID); err != nil {
				t.Fatal(err)
			}
			peer, _ := runtimes.Get(current.PluginID)
			if fail {
				if err := os.WriteFile(filepath.Join(stale.PackageRootPath, "artifact.json"), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := controller.RefreshManagedRuntimeEnvironment(t.Context()); err != nil {
				t.Fatal(err)
			}
			controller.workers.Wait()
			jobs := registry.List()
			if len(jobs) != 1 || jobs[0].TaskType != "plugin.reload" {
				t.Fatalf("reload tasks = %#v", jobs)
			}
			refreshed, _ := runtimes.Get(stale.PluginID)
			if fail {
				if jobs[0].Status != tasks.StatusFailed || jobs[0].Error == nil || refreshed != old || old.Snapshot().State != pluginruntime.StateRunning {
					t.Fatalf("reload failure lost old runtime or diagnostics: task=%#v runtime=%#v", jobs[0], refreshed.Snapshot())
				}
			} else {
				if jobs[0].Status != tasks.StatusSucceeded || refreshed == old || refreshed.Snapshot().State != pluginruntime.StateRunning || old.Snapshot().State != pluginruntime.StateStopped {
					t.Fatalf("reload did not replace runtime: task=%#v runtime=%#v", jobs[0], refreshed.Snapshot())
				}
				for tool, path := range readTools() {
					want := filepath.Join(root, ".deps", "store", "ffmpeg-"+deps.CurrentPlatform(), "9.0.1", "bin", tool)
					if path != want {
						t.Fatalf("child %s path = %q, want %q", tool, path, want)
					}
				}
				if err := controller.RefreshManagedRuntimeEnvironment(t.Context()); err != nil {
					t.Fatal(err)
				}
				controller.workers.Wait()
				if len(registry.List()) != 1 {
					t.Fatal("repeated preparation reloaded current paths")
				}
			}
			if actual, _ := runtimes.Get(current.PluginID); actual != peer {
				t.Fatal("already current plugin reloaded")
			}
			for _, snapshot := range []plugins.Snapshot{disabled, stopped, recovery} {
				if _, ok := runtimes.Get(snapshot.PluginID); ok {
					t.Fatalf("inactive plugin %s was started", snapshot.PluginID)
				}
			}
		})
	}
}
