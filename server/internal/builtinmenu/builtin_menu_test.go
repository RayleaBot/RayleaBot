package builtinmenu

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestBuiltinRootMenuUsesConfiguredPrefixes(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"帮助"}, Prefixes: []string{"#"}}}}
	service := New(Deps{CurrentConfig: func() config.Config { return cfg }, Plugins: plugincatalog.New([]plugins.Snapshot{{
		PluginID: "fortune", Name: "运势", Description: "今日运势", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Commands: []plugins.Command{{ID: "fortune", Name: "fortune", DisplayName: "运势", TriggerType: "exact", TriggerNames: []string{"fortune"}, Description: "今日运势", Usage: "/fortune", Permission: "everyone"}},
	}})})
	payload := service.buildBuiltinMenuData(onebot11.NormalizedEvent{ConversationType: "private", ConversationID: "10002", SenderID: "10002"}, "")
	if got := payload.Data["command_prefixes"]; !reflect.DeepEqual(got, []string{"#"}) {
		t.Fatalf("prefixes = %#v", got)
	}
	if got := payload.Data["trigger_examples"]; !reflect.DeepEqual(got, []string{"#帮助 运势"}) {
		t.Fatalf("trigger_examples = %#v", got)
	}
}

func TestBuildBuiltinCommandsProjectsUnifiedTriggers(t *testing.T) {
	t.Parallel()
	commands := []plugins.CommandView{
		{ID: "list", Name: "角色列表", EffectiveName: "角色列表", TriggerType: "pattern", Description: "查看角色", Usage: "*角色列表", Permission: "everyone"},
		{ID: "guide", Name: "攻略", EffectiveName: "攻略", TriggerType: "exact", Description: "查看攻略", Usage: "/攻略 <角色>", Permission: "everyone"},
		{ID: "fortune", Name: "运势", EffectiveName: "今日运势", TriggerType: "setting", Description: "查看运势", Usage: "今日运势 [日期]", Permission: "everyone"},
	}
	items := buildBuiltinCommands(commands, config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"/", "*"}}}})
	if len(items) != 3 {
		t.Fatalf("items = %#v", items)
	}
	for index, want := range []string{"pattern", "exact", "setting"} {
		if items[index]["trigger_type"] != want {
			t.Fatalf("item %d trigger_type = %#v", index, items[index]["trigger_type"])
		}
		if _, exists := items[index]["command_source"]; exists {
			t.Fatalf("legacy command_source leaked: %#v", items[index])
		}
	}
	if items[0]["usage"] != "角色列表" || items[1]["usage_args"] != "<角色>" || items[2]["usage_args"] != "[日期]" {
		t.Fatalf("usage projection = %#v", items)
	}
}

func TestBuiltinHelpContainsOnlyGroupedCommands(t *testing.T) {
	t.Parallel()
	commands := []plugins.CommandView{
		{ID: "status", Name: "订阅状态", EffectiveName: "订阅状态", TriggerType: "exact", Description: "查看状态", Usage: "/订阅状态", Permission: "everyone"},
		{ID: "refresh", Name: "立即检查", EffectiveName: "立即检查", TriggerType: "exact", Description: "立即检查", Usage: "/立即检查", Permission: "super_admin"},
	}
	help := buildBuiltinHelp(&plugins.HelpView{Title: "订阅与解析", Summary: "订阅和解析命令"}, []plugins.CommandGroup{
		{ID: "subscription", Title: "订阅操作", Commands: []string{"status"}},
		{ID: "maintenance", Title: "维护", Commands: []string{"refresh", "missing"}},
	}, commands, config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"#"}}}})
	groups, ok := help["groups"].([]map[string]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("groups = %#v", help["groups"])
	}
	for _, group := range groups {
		items := group["items"].([]map[string]any)
		if len(items) != 1 {
			t.Fatalf("group contains non-command content: %#v", group)
		}
	}
}

func TestBuiltinPluginMenuAvoidsDuplicateGroupedCommands(t *testing.T) {
	t.Parallel()
	commands := []plugins.CommandView{{ID: "status", Name: "订阅状态", EffectiveName: "订阅状态", TriggerType: "exact", Description: "查看状态", Usage: "/订阅状态", Permission: "everyone"}}
	data := builtinPluginMenuData(map[string]any{
		"name":     "订阅与解析",
		"commands": buildBuiltinCommands(commands, config.Config{}),
		"help":     buildBuiltinHelp(&plugins.HelpView{}, []plugins.CommandGroup{{ID: "subscription", Title: "订阅", Commands: []string{"status"}}}, commands, config.Config{}),
	}, config.Config{})
	groups := data["groups"].([]map[string]any)
	if len(groups) != 1 || groups[0]["title"] != "订阅" {
		t.Fatalf("duplicate command groups = %#v", groups)
	}
}

func TestVisibleBuiltinHelpRequiresVisibleCommands(t *testing.T) {
	t.Parallel()
	help := &plugins.HelpView{Title: "帮助", Summary: "命令说明"}
	if visibleBuiltinHelp(help, nil) != nil {
		t.Fatal("help without visible commands must be hidden")
	}
	commands := []plugins.CommandView{{ID: "status", Name: "状态", Permission: "everyone"}}
	if visibleBuiltinHelp(help, commands) == nil {
		t.Fatal("help with visible commands must remain visible")
	}
}

func TestBuiltinUsageHelpers(t *testing.T) {
	t.Parallel()
	if got := builtinPatternCommandUsage("*<角色>攻略", []string{"/", "*"}); got != "<角色>攻略" {
		t.Fatalf("pattern usage = %q", got)
	}
	want := []map[string]any{{"kind": "required", "text": "角色"}, {"kind": "literal", "text": "攻略"}}
	if got := builtinUsageParts("<角色>攻略", "literal"); !reflect.DeepEqual(got, want) {
		t.Fatalf("usage parts = %#v", got)
	}
}
