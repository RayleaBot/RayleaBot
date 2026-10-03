package plugins

import (
	"reflect"
	"testing"
)

type legacyDisplayCatalog []Snapshot

func (c legacyDisplayCatalog) List() []Snapshot {
	items := make([]Snapshot, 0, len(c))
	for _, snapshot := range c {
		items = append(items, CloneSnapshot(snapshot))
	}
	return items
}

func (c legacyDisplayCatalog) Get(id string) (Snapshot, bool) {
	for _, snapshot := range c {
		if snapshot.PluginID == id {
			return CloneSnapshot(snapshot), true
		}
	}
	return Snapshot{}, false
}

func TestDisplayReadsSupportLegacyCatalogs(t *testing.T) {
	t.Parallel()
	catalog := legacyDisplayCatalog{{PluginID: "fixture", Valid: true, RegistrationState: "installed", RuntimeState: "running", Commands: []Command{{Name: "echo", Aliases: []string{"say"}}}, DefaultConfig: map[string]any{"key": "value"}}}
	if got := ReadDisplaySnapshots(catalog); !reflect.DeepEqual(got, catalog.List()) {
		t.Fatalf("list fallback changed its snapshot: %+v", got)
	}
	got, ok := ReadDisplaySnapshot(catalog, "fixture")
	want, _ := catalog.Get("fixture")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("get fallback changed its snapshot: %+v, %v", got, ok)
	}
	if _, ok := ReadDisplaySnapshot(catalog, "missing"); ok {
		t.Fatal("missing plugin became present")
	}
	if got := ReadCatalogStateCounts(catalog); got != (CatalogStateCounts{Total: 1, Running: 1}) || ReadCatalogCount(catalog) != 1 {
		t.Fatalf("legacy state counts: %+v", got)
	}
}

func TestCommandConflictDomainsPreserveDeclarations(t *testing.T) {
	t.Parallel()
	plugin := func(id string, prefixes CommandPrefixes, commands ...Command) Snapshot {
		return Snapshot{PluginID: id, Valid: true, RegistrationState: "installed", DesiredState: "disabled", CommandPrefixes: prefixes, Commands: commands}
	}
	word := Command{Name: " Echo ", Aliases: []string{"ECHO", " say "}}
	for _, test := range []struct {
		name  string
		items []Snapshot
		want  map[string][]string
	}{
		{"disabled global declarations", []Snapshot{plugin("a", CommandPrefixes{}, word), plugin("b", CommandPrefixes{}, word)}, map[string][]string{"a": {"echo", "say"}, "b": {"echo", "say"}}},
		{"global and dedicated remain separate", []Snapshot{plugin("a", CommandPrefixes{}, word), plugin("b", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"/"}}, word)}, map[string][]string{}},
		{"duplicates do not create a second owner", []Snapshot{plugin("a", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"*", "*"}}, word), plugin("b", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"!"}}, word)}, map[string][]string{}},
		{"shared empty dedicated prefix", []Snapshot{plugin("", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"", ""}}, word), plugin("b", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{""}}, word)}, map[string][]string{"": {"echo", "say"}, "b": {"echo", "say"}}},
		{"prefixes retain case and whitespace", []Snapshot{plugin("a", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"A", " x"}}, word), plugin("b", CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"a", "x"}}, word)}, map[string][]string{}},
		{"pattern exclusion uses match pattern", []Snapshot{plugin("a", CommandPrefixes{}, Command{Name: "echo", TriggerType: "pattern", MatchPattern: " "}), plugin("b", CommandPrefixes{}, Command{Name: "echo"}), plugin("c", CommandPrefixes{}, Command{Name: "echo", Aliases: []string{"echo"}, MatchPattern: "^echo$"})}, map[string][]string{"a": {"echo"}, "b": {"echo"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := DetectCommandConflicts(test.items); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("conflicts=%v, want %v", got, test.want)
			}
		})
	}
}

func TestCommandConflictsKeepLastEligibleDuplicateReach(t *testing.T) {
	t.Parallel()
	items := []Snapshot{
		{PluginID: "a", Valid: true, RegistrationState: "installed", Commands: []Command{{Name: "first"}}},
		{PluginID: "a", Valid: true, RegistrationState: "installed", CommandPrefixes: CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"@"}}, Commands: []Command{{Name: "second"}}},
		{PluginID: "a", Valid: false, RegistrationState: "installed", CommandPrefixes: CommandPrefixes{}, Commands: []Command{{Name: "invalid"}}},
		{PluginID: "b", Valid: true, RegistrationState: "installed", Commands: []Command{{Name: "first", Aliases: []string{"second"}}}},
		{PluginID: "c", Valid: true, RegistrationState: "installed", CommandPrefixes: CommandPrefixes{IgnoreGlobal: true, Dedicated: []string{"@"}}, Commands: []Command{{Name: "FIRST", Aliases: []string{"SECOND"}}}},
		{PluginID: "removed", Valid: true, RegistrationState: "removed", Commands: []Command{{Name: "first", Aliases: []string{"second"}}}},
	}
	want := map[string][]string{"a": {"first", "second"}, "c": {"first", "second"}}
	if got := DetectCommandConflicts(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("conflicts=%v, want %v", got, want)
	}
}
