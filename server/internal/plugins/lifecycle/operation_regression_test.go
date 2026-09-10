package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestEnsurePluginRunningCanceledDuringLifecycleOperationDoesNotChangeState(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	app := newTestAppState(config.Config{}, logger)
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID: "blocked-plugin", DesiredState: "enabled", RegistrationState: "installed", RuntimeState: "stopped",
	}})
	runtimes := newRuntimeRegistry(logger, pluginruntime.Options{})
	app.setTestLifecycle(catalog, nil, runtimes, nil, nil, nil, nil)
	controller := app.services.pluginLifecycle
	release, err := controller.acquireOperation(context.Background(), "blocked-plugin")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := controller.EnsurePluginRunning(ctx, "blocked-plugin"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ensure canceled while another operation owns the plugin: %v", err)
	}
	snapshot, _ := catalog.Get("blocked-plugin")
	if snapshot.RuntimeState != "stopped" {
		t.Fatalf("canceled ensure changed runtime state to %q", snapshot.RuntimeState)
	}
	if _, ok := runtimes.Get("blocked-plugin"); ok {
		t.Fatal("canceled ensure created a runtime during another lifecycle operation")
	}
}

func TestDisableWaitsForLifecycleOperationBeforeShutdownBudget(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	catalog := plugincatalog.New([]plugins.Snapshot{{PluginID: "fixture", DesiredState: "enabled", RegistrationState: "installed", Valid: true}})
	runtimes := newRuntimeRegistry(logger, pluginruntime.Options{})
	manager := runtimes.GetOrCreate("fixture")
	dispatcher := dispatch.New(logger, nil, nil, 4)
	app := newTestAppState(config.Config{}, logger)
	app.setTestLifecycle(catalog, nil, runtimes, dispatcher, nil, nil, nil)
	controller := app.services.pluginLifecycle
	lifecycleCtx, lifecycleCancel := context.WithCancel(t.Context())
	controller.BindLifecycleContext(lifecycleCtx)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(t.Context(), pluginruntime.Spec{
		PluginID: "fixture", Command: executable, Args: []string{"-test.run=^TestLifecycleShutdownProcess$"},
		Env: []string{"RAYLEABOT_LIFECYCLE_SHUTDOWN_FIXTURE=1"}, WorkDir: t.TempDir(),
		InitTimeout: 3 * time.Second, EventTimeout: time.Second, ShutdownGrace: time.Second,
	}, pluginruntime.InitPayload{Timezone: "Asia/Shanghai", CommandPrefixes: []string{"/"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lifecycleCancel()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Stop(ctx)
		dispatcher.Close()
	})
	dispatcher.Register("fixture", manager, nil, nil, 1)
	// A slow reload holds the operation gate while the current runtime serves.
	release, err := controller.acquireOperation(t.Context(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	if _, err := controller.Disable(t.Context(), "fixture"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5250 * time.Millisecond)
	release()
	release = nil
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, exists := runtimes.Get("fixture")
		if !exists && !dispatcher.HasPlugin("fixture") && manager.Snapshot().State == pluginruntime.StateStopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("accepted disable was lost while waiting for the lifecycle gate")
}

func TestLifecycleShutdownProcess(t *testing.T) {
	if os.Getenv("RAYLEABOT_LIFECYCLE_SHUTDOWN_FIXTURE") != "1" {
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
			if encoder.Encode(map[string]any{"type": "init_ack", "request_id": frame["request_id"], "status": "ready"}) != nil {
				os.Exit(2)
			}
		case "shutdown":
			os.Exit(0)
		}
	}
}
