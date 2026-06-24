package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
)

var errPersistFailure = errors.New("persist failure")

// TestHandleCrashDeadLetterCleansUpWebhooks ensures that when a plugin
// exhausts crash retries, the lifecycle controller marks it dead_letter
// and removes any webhook routes the plugin had registered. Otherwise
// webhook routes would keep accepting requests for a plugin that the
// platform has stopped restarting.
func TestHandleCrashDeadLetterCleansUpWebhooks(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	application := newTestAppState(config.Config{}, logger)

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "repo-watcher",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	dispatcher := dispatch.New(logger, nil, nil, 16)
	registry := pluginwebhook.NewRegistry()
	registry.Register(pluginwebhook.Registration{
		PluginID:     "repo-watcher",
		Route:        "github",
		Methods:      []string{"POST"},
		AuthStrategy: "fixed_token",
		Header:       "X-Token",
		SecretRef:    "secret_repo",
		ReplayProtection: pluginwebhook.ReplayProtection{
			TimestampHeader:  "X-Timestamp",
			EventIDHeader:    "X-Event-Id",
			ToleranceSeconds: 300,
			Enforce:          true,
		},
	})
	runtimes := newRuntimeRegistry(logger, runtimemanager.Options{})
	manager := runtimes.GetOrCreate("repo-watcher")
	if manager == nil {
		t.Fatal("expected runtime manager")
	}

	application.setTestLifecycle(catalog, nil, runtimes, dispatcher, nil, nil, registry)

	if _, ok := registry.Get("repo-watcher", "github"); !ok {
		t.Fatal("seed registration was not stored")
	}

	application.services.pluginLifecycle.handleCrash("repo-watcher", runtimemanager.DefaultMaxCrashRetries, "plugin.internal_error")

	snapshot := manager.Snapshot()
	if snapshot.State != runtimemanager.StateDeadLetter {
		t.Fatalf("runtime state = %q, want %q", snapshot.State, runtimemanager.StateDeadLetter)
	}
	if snapshot.EnteredDeadLetterAt == nil {
		t.Fatal("EnteredDeadLetterAt was not recorded after dead_letter entry")
	}
	if _, ok := registry.Get("repo-watcher", "github"); ok {
		t.Fatal("webhook registration should be removed when entering dead_letter")
	}

	if got, ok := catalog.Get("repo-watcher"); !ok || got.RuntimeState != string(runtimemanager.StateDeadLetter) {
		t.Fatalf("catalog runtime_state = %q ok=%v, want dead_letter", got.RuntimeState, ok)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// TestRecoverFromDeadLetterRejectsRunning verifies the controller refuses
// to recover a plugin that is not currently in dead_letter.
func TestRecoverFromDeadLetterRejectsRunning(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	application := newTestAppState(config.Config{}, logger)

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
	}})
	dispatcher := dispatch.New(logger, nil, nil, 16)
	registry := pluginwebhook.NewRegistry()
	runtimes := newRuntimeRegistry(logger, runtimemanager.Options{})
	manager := runtimes.GetOrCreate("weather")
	if manager == nil {
		t.Fatal("expected runtime manager")
	}
	// Plugin runtime is in default Stopped state, not dead_letter.

	application.setTestLifecycle(catalog, nil, runtimes, dispatcher, nil, nil, registry)

	_, err := application.services.pluginLifecycle.RecoverFromDeadLetter(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected error when plugin is not in dead_letter")
	}
	if err.Error() != plugins.ErrPluginNotInDeadLetter.Error() {
		t.Fatalf("err = %v, want ErrPluginNotInDeadLetter", err)
	}
}

// failingDesiredStateRepo is a tiny stub that returns an error from
// SaveDesiredState so RecoverFromDeadLetter can be exercised under a
// persistence failure.
type failingDesiredStateRepo struct {
	saveErr error
}

func (r *failingDesiredStateRepo) LoadDesiredStates(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r *failingDesiredStateRepo) SaveDesiredState(context.Context, string, string, time.Time) error {
	return r.saveErr
}

func (r *failingDesiredStateRepo) DeleteDesiredState(context.Context, string) error {
	return nil
}

// TestRecoverFromDeadLetterPersistFailureLeavesManagerInDeadLetter ensures
// that when desired_state persistence fails during recovery, the runtime
// manager remains in dead_letter so a retry can attempt recovery again.
// Resetting the manager up front would leave the manager stopped while
// the catalog still showed dead_letter, which would cause the next
// recovery call to fail with plugin.not_in_dead_letter.
func TestRecoverFromDeadLetterPersistFailureLeavesManagerInDeadLetter(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	application := newTestAppState(config.Config{}, logger)

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled", // recovery must persist desired_state=enabled
		RuntimeState:      string(runtimemanager.StateDeadLetter),
		DisplayState:      string(runtimemanager.StateDeadLetter),
	}})
	dispatcher := dispatch.New(logger, nil, nil, 16)
	registry := pluginwebhook.NewRegistry()
	runtimes := newRuntimeRegistry(logger, runtimemanager.Options{})
	manager := runtimes.GetOrCreate("weather")
	if manager == nil {
		t.Fatal("expected runtime manager")
	}
	manager.SetDeadLetterState()

	repo := &failingDesiredStateRepo{saveErr: errPersistFailure}

	application.setTestLifecycle(catalog, repo, runtimes, dispatcher, nil, nil, registry)

	_, err := application.services.pluginLifecycle.RecoverFromDeadLetter(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected error when desired_state persistence fails")
	}

	snapshot := manager.Snapshot()
	if snapshot.State != runtimemanager.StateDeadLetter {
		t.Fatalf("manager state after persist failure = %q, want dead_letter", snapshot.State)
	}
	if got, ok := catalog.Get("weather"); !ok || got.DesiredState != "disabled" {
		t.Fatalf("catalog desired_state = %q ok=%v, want disabled", got.DesiredState, ok)
	}
	if got, ok := catalog.Get("weather"); !ok || got.RuntimeState != string(runtimemanager.StateDeadLetter) {
		t.Fatalf("catalog runtime_state = %q ok=%v, want dead_letter", got.RuntimeState, ok)
	}
}
