package plugins

import (
	"reflect"
	"testing"
)

func TestResolveCommandMatches(t *testing.T) {
	global := []string{"/", "*", "~"}
	exact := func(id, name string) Command { return Command{ID: id, Name: name, TriggerType: "exact"} }
	pattern := func(id, expr string) Command {
		return Command{ID: id, TriggerType: "pattern", MatchPattern: expr}
	}
	genshin := CommandEntry{PluginID: "genshin", Commands: []Command{exact("note", "体力"), pattern("panel", `^.+面板$`)}}
	starrail := CommandEntry{PluginID: "starrail", Commands: []Command{exact("note", "体力"), pattern("panel", `^.+面板$`)},
		Prefixes: CommandPrefixes{Dedicated: SortCommandPrefixes([]string{"*", "星", "星穹铁道", "星铁"}), IgnoreGlobal: true}}
	zzz := CommandEntry{PluginID: "zzz", Commands: []Command{exact("note", "体力")},
		Prefixes: CommandPrefixes{Dedicated: []string{"%"}, IgnoreGlobal: true}}
	echo := CommandEntry{PluginID: "echo", Commands: []Command{exact("echo", "echo")}}
	rival := CommandEntry{PluginID: "rival", Commands: []Command{exact("note", "体力")},
		Prefixes: CommandPrefixes{Dedicated: []string{"*"}}}
	entries := []CommandEntry{genshin, starrail, zzz, echo}

	type summary struct {
		Plugin  string
		Tier    CommandTier
		Prefix  string
		Command string
		Args    []string
	}
	for _, scenario := range []struct {
		name    string
		entries []CommandEntry
		text    string
		want    []summary
	}{
		{"global prefix reaches a plugin without a declaration", entries, "/体力", []summary{{"genshin", CommandTierGlobal, "/", "体力", nil}}},
		{"dedicated prefix shadows the same character used as a global prefix", entries, "*体力", []summary{{"starrail", CommandTierDedicated, "*", "体力", nil}}},
		{"a shared character stays global for commands nobody dedicated", entries, "*echo hi", []summary{{"echo", CommandTierGlobal, "*", "echo", []string{"hi"}}}},
		{"a prefix outside the global list still addresses its plugin", entries, "%体力", []summary{{"zzz", CommandTierDedicated, "%", "体力", nil}}},
		{"bare word prefix", entries, "星铁体力 100000001", []summary{{"starrail", CommandTierDedicated, "星铁", "体力", []string{"100000001"}}}},
		{"dedicated prefix after the global prefix", entries, "/星铁体力", []summary{{"starrail", CommandTierDedicated, "/星铁", "体力", nil}}},
		{"longest dedicated prefix decides the pattern command name", entries, "星穹铁道希儿面板", []summary{{"starrail", CommandTierDedicated, "星穹铁道", "希儿面板", nil}}},
		{"a shorter prefix applies when the longer one leaves no command", entries, "星体力", []summary{{"starrail", CommandTierDedicated, "星", "体力", nil}}},
		{"plugins sharing a dedicated prefix and command both survive", []CommandEntry{genshin, starrail, rival}, "*体力", []summary{{"starrail", CommandTierDedicated, "*", "体力", nil}, {"rival", CommandTierDedicated, "*", "体力", nil}}},
		{"a plugin that keeps the global prefixes matches through them too", []CommandEntry{rival}, "/体力", []summary{{"rival", CommandTierGlobal, "/", "体力", nil}}},
		{"ordinary chat that starts with a prefix word", entries, "星铁真好玩", nil},
		{"prefix without a command", entries, "*", nil},
		{"no prefix", entries, "体力", nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var got []summary
			for _, match := range ResolveCommandMatches(scenario.entries, scenario.text, global) {
				args := match.Args
				if len(args) == 0 {
					args = nil
				}
				got = append(got, summary{match.PluginID, match.Tier, match.Prefix, match.Command, args})
			}
			if !reflect.DeepEqual(got, scenario.want) {
				t.Fatalf("matches = %+v, want %+v", got, scenario.want)
			}
		})
	}
}

func TestEffectivePrefixesListDedicatedFirst(t *testing.T) {
	global := []string{"/", "*"}
	if got := (CommandPrefixes{Dedicated: []string{"星铁", "*"}}).EffectivePrefixes(global); !reflect.DeepEqual(got, []string{"星铁", "*", "/"}) {
		t.Fatalf("accepting plugin = %v", got)
	}
	if got := (CommandPrefixes{Dedicated: []string{"%"}, IgnoreGlobal: true}).EffectivePrefixes(global); !reflect.DeepEqual(got, []string{"%"}) {
		t.Fatalf("ignoring plugin = %v", got)
	}
	if got := (CommandPrefixes{}).EffectivePrefixes(global); !reflect.DeepEqual(got, global) {
		t.Fatalf("undeclared plugin = %v", got)
	}
}

func TestCommandConflictsFollowPrefixReach(t *testing.T) {
	plugin := func(id string, prefixes CommandPrefixes) Snapshot {
		return Snapshot{PluginID: id, Valid: true, RegistrationState: "installed", Commands: []Command{{Name: "体力"}}, CommandPrefixes: prefixes}
	}
	conflicts := DetectCommandConflicts([]Snapshot{
		plugin("genshin", CommandPrefixes{}),
		plugin("starrail", CommandPrefixes{Dedicated: []string{"*"}, IgnoreGlobal: true}),
		plugin("zzz", CommandPrefixes{Dedicated: []string{"%"}, IgnoreGlobal: true}),
		plugin("other-global", CommandPrefixes{Dedicated: []string{"!"}}),
		plugin("other-star", CommandPrefixes{Dedicated: []string{"*"}, IgnoreGlobal: true}),
	})
	want := map[string][]string{"genshin": {"体力"}, "other-global": {"体力"}, "starrail": {"体力"}, "other-star": {"体力"}}
	if !reflect.DeepEqual(conflicts, want) {
		t.Fatalf("conflicts = %v, want %v", conflicts, want)
	}
}

func TestCommandPrefixViewKeepsOrderAndOwnsSlices(t *testing.T) {
	prefixes := CommandPrefixes{Dedicated: []string{"*", "星铁", "/"}}
	global := []string{"/", "!"}
	view := BuildCommandPrefixView(prefixes, global)
	if !reflect.DeepEqual(view.All, []string{"*", "星铁", "/", "!"}) || !reflect.DeepEqual(view.Dedicated, prefixes.Dedicated) {
		t.Fatalf("前缀视图 = %+v", view)
	}
	view.All[0], view.Dedicated[1] = "changed-all", "changed-dedicated"
	if !reflect.DeepEqual(prefixes.Dedicated, []string{"*", "星铁", "/"}) || !reflect.DeepEqual(global, []string{"/", "!"}) {
		t.Fatalf("视图修改影响来源：prefixes=%+v global=%v", prefixes, global)
	}
	if view.Dedicated[0] != "*" || view.All[1] != "星铁" {
		t.Fatalf("视图切片共享了可变数据：%+v", view)
	}
}

func TestCommandPrefixViewEmptyCollections(t *testing.T) {
	view := BuildCommandPrefixView(CommandPrefixes{}, []string{"/"})
	if !reflect.DeepEqual(view.All, []string{"/"}) || !reflect.DeepEqual(view.Dedicated, []string{}) {
		t.Fatalf("仅通用前缀视图 = %+v", view)
	}
	view = BuildCommandPrefixView(CommandPrefixes{IgnoreGlobal: true}, []string{"/"})
	if !reflect.DeepEqual(view.All, []string{}) || !reflect.DeepEqual(view.Dedicated, []string{}) {
		t.Fatalf("无可用前缀时应返回空数组：%+v", view)
	}
}
