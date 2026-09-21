package catalog

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestProjectCommandPrefixes(t *testing.T) {
	declared := func(acceptGlobal bool) plugins.Snapshot {
		return plugins.Snapshot{ManifestCommandPrefixes: &plugins.ManifestCommandPrefixes{
			Dedicated: []string{"*", "星铁"}, SettingsKey: "command_prefixes", AcceptGlobal: acceptGlobal,
		}}
	}
	for _, scenario := range []struct {
		name     string
		snapshot plugins.Snapshot
		settings map[string]any
		want     plugins.CommandPrefixes
	}{
		{"no declaration keeps the global prefixes only", plugins.Snapshot{}, map[string]any{"command_prefixes": []any{"*"}}, plugins.CommandPrefixes{}},
		{"manifest defaults keep the declared order", declared(false), nil, plugins.CommandPrefixes{Dedicated: []string{"*", "星铁"}, IgnoreGlobal: true}},
		{"a saved list replaces the defaults", declared(false), map[string]any{"command_prefixes": []any{"sr", " ", "星穹铁道"}}, plugins.CommandPrefixes{Dedicated: []string{"sr", "星穹铁道"}, IgnoreGlobal: true}},
		{"a saved string is one prefix", declared(true), map[string]any{"command_prefixes": "崩铁"}, plugins.CommandPrefixes{Dedicated: []string{"崩铁"}}},
		{"an empty saved list cannot make an exclusive plugin unreachable", declared(false), map[string]any{"command_prefixes": []any{}}, plugins.CommandPrefixes{Dedicated: []string{"*", "星铁"}, IgnoreGlobal: true}},
		{"an empty saved list drops dedicated prefixes when global ones remain", declared(true), map[string]any{"command_prefixes": []any{}}, plugins.CommandPrefixes{Dedicated: []string{}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got := ProjectCommandPrefixes(scenario.snapshot, scenario.settings)
			if len(got.Dedicated) == 0 && len(scenario.want.Dedicated) == 0 {
				got.Dedicated, scenario.want.Dedicated = nil, nil
			}
			if !reflect.DeepEqual(got, scenario.want) {
				t.Fatalf("prefixes = %+v, want %+v", got, scenario.want)
			}
		})
	}
}
