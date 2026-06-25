package actions_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	defaultactionmodules "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/defaultmodules"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestExecuteConfigReadWriteRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginconfig.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	service := actions.New(actions.Deps{
		Capabilities: &stubCapabilityView{capabilities: map[string]bool{
			"config.read":  true,
			"config.write": true,
		}},
		PluginConfig: repo,
		Registrars:   defaultactionmodules.Registrars(),
	})

	if _, err := repo.SeedDefaults(context.Background(), "weather", map[string]any{
		"default_city": "北京",
		"unit":         "celsius",
	}); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	readResult, err := service.Execute(context.Background(), "weather", "req_config_1", runtimeaction.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit", "missing"},
	}, runtimeprotocol.Event{})
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

	writeResult, err := service.Execute(context.Background(), "weather", "req_config_2", runtimeaction.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
			"unit":         "fahrenheit",
		},
	}, runtimeprotocol.Event{})
	if err != nil {
		t.Fatalf("config.write failed: %v", err)
	}
	changedKeys, _ := writeResult["changed_keys"].([]string)
	if len(changedKeys) != 2 || changedKeys[0] != "default_city" || changedKeys[1] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", writeResult["changed_keys"])
	}

	readResult, err = service.Execute(context.Background(), "weather", "req_config_3", runtimeaction.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit"},
	}, runtimeprotocol.Event{})
	if err != nil {
		t.Fatalf("config.read second call failed: %v", err)
	}
	values, _ = readResult["values"].(map[string]any)
	if values["default_city"] != "上海" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated config values: %#v", values)
	}
}
