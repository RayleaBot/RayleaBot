package actions

import (
	"context"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

type legacyPluginListCatalog struct{ catalog *catalog.Catalog }

func (c legacyPluginListCatalog) List() []plugins.Snapshot { return c.catalog.List() }

func TestPluginListDisplayProjectionMatchesLegacyCatalog(t *testing.T) {
	t.Parallel()
	registry := catalog.New([]plugins.Snapshot{
		{PluginID: "a", Valid: true, RegistrationState: "installed", DesiredState: "disabled", Commands: []plugins.Command{{ID: "echo", Name: "echo", DisplayName: "Echo", Aliases: []string{"say"}, TriggerType: "exact", TriggerNames: []string{"echo"}}}, CommandGroups: []plugins.CommandGroup{{ID: "main", Commands: []string{"echo"}}}, Help: &plugins.Help{Title: "Help"}, DefaultConfig: map[string]any{"unused": []any{"large"}}},
		{PluginID: "b", Valid: true, RegistrationState: "installed", Commands: []plugins.Command{{ID: "echo", Name: "echo", DisplayName: "Echo"}}},
		{PluginID: "invalid", Valid: false, RegistrationState: "installed", DisplayState: "conflict", ConflictPaths: []string{"a/info.json", "b/info.json"}, SourceRoots: []string{"a", "b"}},
	})
	read := func(source PluginCatalog) map[string]any {
		result, err := executePluginList(context.Background(), Deps{Plugins: source}, ActionRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	want := read(legacyPluginListCatalog{catalog: registry})
	got := read(registry)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("display result differs from full catalog: got=%+v want=%+v", got, want)
	}
	items := got["items"].([]map[string]any)
	commands := items[0]["commands"].([]map[string]any)
	commands[0]["effective_names"].([]string)[0] = "changed"
	items[0]["command_conflicts"].([]string)[0] = "changed"
	if !reflect.DeepEqual(read(registry), want) {
		t.Fatal("action response mutation changed later catalog projections")
	}
}
