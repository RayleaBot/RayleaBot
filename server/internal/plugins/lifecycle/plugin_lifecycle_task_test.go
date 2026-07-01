package lifecycle

import (
	"context"
	"log/slog"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestReloadCreatesPluginReloadTask(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestSystem(registry, nil, nil, nil)
	app.setTestLifecycle(
		catalog,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)

	if _, err := app.services.pluginLifecycle.Reload(context.Background(), "weather"); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	created := waitTaskType(t, registry, "plugin.reload")

	final := waitTask(t, registry, created.TaskID, tasks.StatusFailed)
	if final.FinishedAt == nil {
		t.Fatal("plugin.reload task missing finished_at")
	}
	if final.Error == nil || final.Error.Code != "platform.invalid_request" {
		t.Fatalf("task error = %#v, want platform.invalid_request", final.Error)
	}
	if final.Error.Details["plugin_id"] != "weather" {
		t.Fatalf("task error details = %#v, want plugin_id weather", final.Error.Details)
	}
}

func TestReloadRejectedBeforeAcceptanceDoesNotCreateTask(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestSystem(registry, nil, nil, nil)
	app.setTestLifecycle(
		catalog,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)

	if _, err := app.services.pluginLifecycle.Reload(context.Background(), "weather"); err == nil {
		t.Fatal("expected Reload to reject disabled plugin")
	}
	if tasks := registry.List(); len(tasks) != 0 {
		t.Fatalf("tasks = %#v, want none", tasks)
	}
}

func waitTaskType(t *testing.T, registry *tasks.Registry, taskType string) tasks.Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, snapshot := range registry.List() {
			if snapshot.TaskType == taskType {
				return snapshot
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task type %s was not created; tasks: %#v", taskType, registry.List())
	return tasks.Snapshot{}
}
