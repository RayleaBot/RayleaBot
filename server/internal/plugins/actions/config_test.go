package actions_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestExecuteConfigReadWriteRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	service := actions.New(actions.Deps{
		Capabilities: &stubCapabilityView{capabilities: map[string]bool{
			"config.read":  true,
			"config.write": true,
		}},
		PluginConfig: repo,
	})

	if _, err := repo.SeedDefaults(context.Background(), "weather", map[string]any{
		"default_city": "北京",
		"unit":         "celsius",
	}); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	readResult, err := service.Execute(context.Background(), "weather", "req_config_1", pluginruntime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit", "missing"},
	}, pluginruntime.Event{})
	if err != nil {
		t.Fatalf("config.read failed: %v", err)
	}
	values, _ := readResult["values"].(map[string]any)
	if values["default_city"] != "北京" || values["unit"] != "celsius" {
		t.Fatalf("unexpected config.read values: %#v", values)
	}
	if _, ok := values["missing"]; ok {
		t.Fatalf("missing key should not be returned: %#v", values)
	}

	writeResult, err := service.Execute(context.Background(), "weather", "req_config_2", pluginruntime.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
			"unit":         "fahrenheit",
		},
	}, pluginruntime.Event{})
	if err != nil {
		t.Fatalf("config.write failed: %v", err)
	}
	changedKeys, _ := writeResult["changed_keys"].([]string)
	if len(changedKeys) != 2 || changedKeys[0] != "default_city" || changedKeys[1] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", writeResult["changed_keys"])
	}

	readResult, err = service.Execute(context.Background(), "weather", "req_config_3", pluginruntime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit"},
	}, pluginruntime.Event{})
	if err != nil {
		t.Fatalf("config.read second call failed: %v", err)
	}
	values, _ = readResult["values"].(map[string]any)
	if values["default_city"] != "上海" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated config values: %#v", values)
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

	result := actions.ConfigChangedDispatcher(dispatcher)(ctx, "weather")
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

func (r *configChangeRuntime) DeliverEvent(ctx context.Context, event pluginruntime.Event) (pluginruntime.Delivery, error) {
	r.contextErrors <- ctx.Err()
	return pluginruntime.Delivery{Result: map[string]any{"handled": true}}, nil
}

func (r *configChangeRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}
