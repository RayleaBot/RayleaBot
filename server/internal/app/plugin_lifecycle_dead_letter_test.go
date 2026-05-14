package app

import (
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

// TestHandleCrashDeadLetterCleansUpWebhooks ensures that when a plugin
// exhausts crash retries, the lifecycle controller marks it dead_letter
// and removes any webhook routes the plugin had registered. Otherwise
// webhook routes would keep accepting requests for a plugin that the
// platform has stopped restarting.
func TestHandleCrashDeadLetterCleansUpWebhooks(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	application := newTestAppState(config.Config{}, logger)

	catalog := plugins.NewCatalog([]plugins.Snapshot{{
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
	runtimes := newRuntimeRegistry(logger, runtime.Options{})
	manager := runtimes.GetOrCreate("repo-watcher")
	if manager == nil {
		t.Fatal("expected runtime manager")
	}

	application.setTestLifecycle(catalog, nil, nil, runtimes, dispatcher, nil, nil, registry)

	if _, ok := registry.Get("repo-watcher", "github"); !ok {
		t.Fatal("seed registration was not stored")
	}

	application.pluginLifecycle.handleCrash("repo-watcher", runtime.DefaultMaxCrashRetries, "plugin.internal_error")

	snapshot := manager.Snapshot()
	if snapshot.State != runtime.StateDeadLetter {
		t.Fatalf("runtime state = %q, want %q", snapshot.State, runtime.StateDeadLetter)
	}
	if snapshot.EnteredDeadLetterAt == nil {
		t.Fatal("EnteredDeadLetterAt was not recorded after dead_letter entry")
	}
	if _, ok := registry.Get("repo-watcher", "github"); ok {
		t.Fatal("webhook registration should be removed when entering dead_letter")
	}

	if got, ok := catalog.Get("repo-watcher"); !ok || got.RuntimeState != string(runtime.StateDeadLetter) {
		t.Fatalf("catalog runtime_state = %q ok=%v, want dead_letter", got.RuntimeState, ok)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
