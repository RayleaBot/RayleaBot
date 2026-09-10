package actions_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestExecuteConfigWriteUsesImplicitPrivateNamespace(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	pluginCatalog := catalog.New([]plugins.Snapshot{{
		PluginID: "weather",
		DefaultConfig: map[string]any{
			"default_city": "Beijing",
			"unit":         "celsius",
			"timeout":      15,
		},
	}})
	var refreshedSettings map[string]any
	service := actions.New(actions.Deps{
		Plugins:      pluginCatalog,
		PluginConfig: repo,
		RefreshCommands: func(_ context.Context, _ string, settings map[string]any) {
			refreshedSettings = settings
		},
	})

	if _, err := repo.SeedDefaults(context.Background(), "weather", map[string]any{
		"default_city": "Beijing",
		"unit":         "celsius",
	}); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	writeResult, err := service.Execute(context.Background(), "weather", "req_config_2", plugins.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "Shanghai",
			"unit":         "fahrenheit",
		},
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("config.write failed: %v", err)
	}
	changedKeys, _ := writeResult["changed_keys"].([]string)
	if len(changedKeys) != 2 || changedKeys[0] != "default_city" || changedKeys[1] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", writeResult["changed_keys"])
	}

	values, err := repo.ReadAll(context.Background(), "weather")
	if err != nil {
		t.Fatalf("read stored config: %v", err)
	}
	if values["default_city"] != "Shanghai" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated config values: %#v", values)
	}
	if refreshedSettings["default_city"] != "Shanghai" || refreshedSettings["unit"] != "fahrenheit" || refreshedSettings["timeout"] != 15 {
		t.Fatalf("config.write did not refresh commands with the effective snapshot: %#v", refreshedSettings)
	}
}

func TestConfigChangedDispatcherDetachesCallerCancellation(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(slog.Default(), nil, nil, 1)
	defer dispatcher.Close()

	runtime := &configChangeRuntime{contextErrors: make(chan error, 1)}
	dispatcher.Register("weather", runtime, []string{"config.changed"}, nil, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := actions.ConfigChangedDispatcher(dispatcher)(ctx, "weather", map[string]any{"city": "Beijing"}, []string{"city"})
	if !result.Delivered {
		t.Fatalf("config.changed delivery was not admitted: %#v", result)
	}

	select {
	case err := <-runtime.contextErrors:
		if err != nil {
			t.Fatalf("config.changed inherited caller cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("config.changed event was not delivered")
	}
}

type configChangeRuntime struct {
	contextErrors chan error
}

func (r *configChangeRuntime) DeliverEvent(ctx context.Context, event chatevent.Event) (plugins.Delivery, error) {
	r.contextErrors <- ctx.Err()
	return plugins.Delivery{Result: map[string]any{"handled": true}}, nil
}

func (r *configChangeRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func TestConfigRefreshPreservesPatternDirectedDelivery(t *testing.T) {
	t.Parallel()
	dispatcher := dispatch.New(slog.Default(), nil, nil, 8)
	t.Cleanup(dispatcher.Close)
	declaration := plugins.Command{ID: "pattern", DisplayName: "查询", TriggerType: "pattern", MatchPattern: "^ping[0-9]+$"}
	pluginCatalog := catalog.New([]plugins.Snapshot{{
		PluginID: "pattern", Valid: true, RegistrationState: "installed", DesiredState: "enabled",
		ManifestCommands: []plugins.Command{declaration},
	}})
	snapshot, _ := pluginCatalog.Get("pattern")
	commands := catalog.ProjectCommands(snapshot, nil)
	dispatcher.Register("pattern", &configChangeRuntime{contextErrors: make(chan error, 4)}, []string{"message.group"}, commands, 1)
	dispatcher.Register("observer", &configChangeRuntime{contextErrors: make(chan error, 4)}, []string{"message.group"}, nil, 1)
	event := chatevent.Event{EventID: "fixture-event", EventType: "message.group", SourceProtocol: "onebot11", SourceAdapter: "fixture", Timestamp: 1}
	for _, refresh := range []bool{false, true} {
		if refresh {
			actions.RefreshCommands(pluginCatalog, dispatcher)(context.Background(), "pattern", map[string]any{})
		}
		results := dispatcher.Dispatch(context.Background(), event, "ping123")
		if len(results) != 1 || results[0].PluginID != "pattern" || results[0].Outcome != dispatch.OutcomeDelivered {
			t.Fatalf("refresh=%v: expected directed delivery to pattern, got %#v", refresh, results)
		}
	}
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (r *configChangeRuntime) ReadyForEvents() bool {
	return r.Snapshot().State == pluginruntime.StateRunning
}
