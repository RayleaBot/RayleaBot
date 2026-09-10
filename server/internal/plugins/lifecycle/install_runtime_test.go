package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestStartInstalledReturnsInitializationFailureSynchronously(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID: "broken-artifact", Valid: false,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "stopped",
	}})
	runtimes := newRuntimeRegistry(logger, pluginruntime.Options{})
	application := newTestAppState(config.Config{}, logger)
	application.setTestLifecycle(t, catalog, nil, runtimes, dispatch.New(logger, nil, nil, 16), nil, nil, nil)
	if err := application.services.pluginLifecycle.StartInstalled(t.Context(), "broken-artifact"); err == nil {
		t.Fatal("installer received success before a valid runtime initialized")
	}
	if _, exists := runtimes.Get("broken-artifact"); exists {
		t.Fatal("failed initialization retained an unused runtime")
	}
	if got, _ := catalog.Get("broken-artifact"); got.RuntimeState != "stopped" {
		t.Fatalf("failed initialization runtime state = %q", got.RuntimeState)
	}
}

func TestStopAndResetReturnsOperationCancellationWithoutDroppingOwner(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	catalog := plugincatalog.New([]plugins.Snapshot{{PluginID: "weather", DesiredState: "enabled"}})
	runtimes := newRuntimeRegistry(logger, pluginruntime.Options{})
	manager := runtimes.GetOrCreate("weather")
	application := newTestAppState(config.Config{}, logger)
	application.setTestLifecycle(t, catalog, nil, runtimes, dispatch.New(logger, nil, nil, 16), nil, nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := application.services.pluginLifecycle.StopAndResetPluginWithContext(ctx, "weather")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stop error = %v, want canceled", err)
	}
	if got, exists := runtimes.Get("weather"); !exists || got != manager {
		t.Fatal("canceled stop removed the runtime owner")
	}
}
