package lifecycle

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

// Only fixture copies of this executable enter the plugin protocol loop.
func init() {
	mode, err := os.ReadFile(".crash-cycle-probe")
	if err != nil {
		return
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var frame map[string]any
		if decoder.Decode(&frame) != nil {
			os.Exit(0)
		}
		switch frame["type"] {
		case "init":
			file, err := os.OpenFile(filepath.Join(os.Getenv("RAYLEABOT_PLUGIN_DATA_DIR"), "starts"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(2)
			}
			_, err = file.WriteString("start\n")
			_ = file.Close()
			if err != nil || encoder.Encode(map[string]any{"type": "init_ack", "request_id": frame["request_id"], "status": "ready"}) != nil {
				os.Exit(2)
			}
			if string(mode) == "crash" {
				go func() { time.Sleep(200 * time.Millisecond); os.Exit(23) }()
			}
		case "event":
			_ = encoder.Encode(map[string]any{"type": "result", "request_id": frame["request_id"], "status": "success", "data": map[string]any{}})
		case "ping":
			_ = encoder.Encode(map[string]any{"type": "pong", "request_id": frame["request_id"]})
		case "shutdown":
			os.Exit(0)
		}
	}
}

func TestCrashAfterSuccessfulInitReachesDeadLetterWithoutRestartingPeer(t *testing.T) {
	// Probes inherit this environment; race builds otherwise sleep a second
	// in TSan before a zero exit and overrun the shutdown grace.
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	makePlugin := func(id, mode string) plugins.Snapshot {
		t.Helper()
		dir := filepath.Join(root, "plugins", "installed", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		entry := "probe" + filepath.Ext(executable)
		manifest, _ := json.Marshal(map[string]any{"id": id, "name": id, "version": "1.0.0", "manifest_version": "4", "min_core_version": "0.7.0", "license": "MIT", "events": []string{"plugin.started"}})
		artifactDoc, _ := json.Marshal(map[string]any{"artifact_version": "2", "target_platform": platform, "entry": entry})
		for name, data := range map[string][]byte{entry: binary, "info.json": manifest, "artifact.json": artifactDoc, ".crash-cycle-probe": []byte(mode)} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return plugins.Snapshot{PluginID: id, Name: id, ManifestPath: filepath.Join(dir, "info.json"), PackageRootPath: dir,
			Valid: true, RegistrationState: "installed", DesiredState: "enabled", Events: []string{"plugin.started"}}
	}
	broken, healthy := makePlugin("crash-fixture", "crash"), makePlugin("healthy-fixture", "healthy")
	cat := catalog.New([]plugins.Snapshot{broken, healthy})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runtimes := pluginruntime.NewRegistry(logger, pluginruntime.Options{})
	controller := newTestController(t, Deps{RepoRoot: root, PluginDataRoot: filepath.Join(root, "data"), Logger: logger, Plugins: cat, Runtimes: runtimes,
		CurrentConfig: func() config.Config {
			return config.Config{Scheduler: config.SchedulerConfig{Timezone: "UTC"}, Runtime: config.RuntimeConfig{
				// Each probe is a copy of this (possibly race-instrumented) test binary; on a
				// loaded machine a short init timeout turns "crash after init" into "init timed out".
				PluginInitTimeoutSeconds: 10, ShutdownGraceSeconds: 1, CrashBackoffInitialSeconds: 1, CrashBackoffMaxSeconds: 1,
			}}
		}})
	runtimes.SetOnCrash(controller.HandleCrash)
	if err := controller.StartInstalled(t.Context(), healthy.PluginID); err != nil {
		t.Fatal(err)
	}
	peer, _ := runtimes.Get(healthy.PluginID)
	peerPID := peer.Snapshot().PID
	if err := controller.StartInstalled(t.Context(), broken.PluginID); err != nil {
		t.Fatal(err)
	}
	manager, _ := runtimes.Get(broken.PluginID)
	waitForState := func(state pluginruntime.State) {
		t.Helper()
		deadline := time.Now().Add(40 * time.Second)
		for time.Now().Before(deadline) {
			if manager.Snapshot().State == state {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("runtime did not reach %s: %+v", state, manager.Snapshot())
	}
	waitForState(pluginruntime.StateDeadLetter)
	if got := manager.Snapshot().CrashCount; got != pluginruntime.DefaultMaxCrashRetries {
		t.Fatalf("crash count = %d", got)
	}
	starts, err := os.ReadFile(filepath.Join(root, "data", broken.PluginID, "starts"))
	if err != nil || strings.Count(string(starts), "start\n") != pluginruntime.DefaultMaxCrashRetries {
		t.Fatalf("unexpected start history: %q, %v", starts, err)
	}
	if state := peer.Snapshot(); state.State != pluginruntime.StateRunning || state.PID != peerPID {
		t.Fatalf("healthy peer was disturbed: %+v", state)
	}
	// An explicit recovery after repairing the plugin begins a fresh cycle.
	if err := os.WriteFile(filepath.Join(broken.PackageRootPath, ".crash-cycle-probe"), []byte("healthy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.RecoverFromDeadLetter(context.Background(), broken.PluginID); err != nil {
		t.Fatal(err)
	}
	waitForState(pluginruntime.StateRunning)
	if manager.Snapshot().CrashCount != 0 || peer.Snapshot().PID != peerPID {
		t.Fatal("manual recovery did not isolate and reset the failed plugin")
	}
}
