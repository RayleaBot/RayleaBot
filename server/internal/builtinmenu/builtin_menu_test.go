package menu

import (
	"reflect"
	"testing"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestBuiltinRootMenuDataUsesBuiltinMenuPrefixesAndTriggerExamples(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"帮助"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	service := New(Deps{
		CurrentConfig: func() config.Config { return cfg },
		Plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "fortune",
			Name:              "运势",
			Description:       "今日运势",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "fortune",
				Usage:      "/fortune",
				Permission: "everyone",
			}},
		}}),
	})

	payload := service.buildBuiltinMenuData(adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		ConversationType: "private",
		ConversationID:   "10002",
		SenderID:         "10002",
		ActorRole:        "member",
	}, "")
	data := payload.Data
	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"#"}) {
		t.Fatalf("command_prefixes = %#v, want [#]", got)
	}
	if got := data["trigger_examples"]; !reflect.DeepEqual(got, []string{"#帮助 运势"}) {
		t.Fatalf("trigger_examples = %#v, want [#帮助 运势]", got)
	}
	items, ok := data["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected root menu items: %#v", data["items"])
	}
	if _, ok := items[0]["usage"]; ok {
		t.Fatalf("root menu item should not include usage: %#v", items[0])
	}
}

func TestBuiltinRootMenuDataFallsBackToCommandPrefixes(t *testing.T) {
	t.Parallel()

	data := builtinRootMenuData([]map[string]any{{
		"id":          "echo",
		"name":        "Echo",
		"description": "复读消息",
	}}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"X", "C"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
		}},
	})

	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"X", "C"}) {
		t.Fatalf("command_prefixes = %#v, want [X C]", got)
	}
	if got := data["trigger_examples"]; !reflect.DeepEqual(got, []string{"Xhelp Echo", "CEcho帮助"}) {
		t.Fatalf("trigger_examples = %#v, want [Xhelp Echo CEcho帮助]", got)
	}
}

func TestBuiltinPluginMenuDataUsesMenuPrefixesWithoutTriggerExamples(t *testing.T) {
	t.Parallel()

	data := builtinPluginMenuData(map[string]any{
		"id":             "subscription-hub",
		"name":           "订阅中心",
		"plugin_name":    "订阅中心",
		"plugin_version": "0.1.0",
		"description":    "订阅平台内容并推送更新",
		"commands": buildBuiltinCommands([]plugins.CommandView{{
			Name:        "全部b站订阅列表",
			Description: "查看所有群聊和私聊的 Bilibili 订阅列表",
			Usage:       "/全部b站订阅列表",
			Permission:  "super_admin",
		}}, config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"#", "*"}}}}),
	}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Prefixes: []string{"#", "*"},
		}},
	})

	if _, ok := data["trigger_examples"]; ok {
		t.Fatalf("plugin menu should not include trigger_examples: %#v", data)
	}
	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
		t.Fatalf("plugin command_prefixes = %#v, want [# *]", got)
	}
	if got := data["plugin_name"]; got != "订阅中心" {
		t.Fatalf("plugin_name = %#v, want 订阅中心", got)
	}
	if got := data["plugin_version"]; got != "0.1.0" {
		t.Fatalf("plugin_version = %#v, want 0.1.0", got)
	}
	groups, ok := data["groups"].([]map[string]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("unexpected plugin menu groups: %#v", data["groups"])
	}
	items, ok := groups[0]["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected command items: %#v", groups[0]["items"])
	}
	if got := items[0]["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
		t.Fatalf("command item prefixes = %#v, want [# *]", got)
	}
	if _, ok := items[0]["usage"]; ok {
		t.Fatalf("command item should not include usage: %#v", items[0])
	}
}

func TestBuiltinPluginMenuDataDoesNotTreatHelpTitleAsCommand(t *testing.T) {
	t.Parallel()

	data := builtinPluginMenuData(map[string]any{
		"id":          "subscription-hub",
		"name":        "订阅中心",
		"description": "订阅平台内容并推送更新",
		"help": buildBuiltinHelp(&plugins.HelpView{
			Groups: []plugins.HelpGroupView{{
				Title: "说明",
				Items: []plugins.HelpItemView{{
					Title:       "订阅管理说明",
					Description: "说明当前订阅配置。",
					Permission:  "group_admin",
				}, {
					Title:       "查看状态",
					Description: "查看订阅状态。",
					Command:     "订阅状态",
					Usage:       "/订阅状态",
					Permission:  "everyone",
				}},
			}},
		}, nil, config.Config{}),
	}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Prefixes: []string{"#", "*"},
		}},
	})

	groups, ok := data["groups"].([]map[string]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("unexpected plugin menu groups: %#v", data["groups"])
	}
	items, ok := groups[0]["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected help items: %#v", groups[0]["items"])
	}
	if _, ok := items[0]["command_prefixes"]; ok {
		t.Fatalf("non-command help item should not include command prefixes: %#v", items[0])
	}
	if got := items[1]["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
		t.Fatalf("command help item prefixes = %#v, want [# *]", got)
	}
	if _, ok := items[1]["usage"]; ok {
		t.Fatalf("command help item should not include usage: %#v", items[1])
	}
	if _, ok := items[1]["command_name"]; ok {
		t.Fatalf("command_name should not leak into render data: %#v", items[1])
	}
}

func TestBuiltinPluginMenuDataOmitsCommandsCoveredByHelp(t *testing.T) {
	t.Parallel()

	data := builtinPluginMenuData(map[string]any{
		"id":          "subscription-hub",
		"name":        "订阅中心",
		"description": "订阅平台内容并推送更新",
		"commands": buildBuiltinCommands([]plugins.CommandView{{
			Name:        "订阅状态",
			Description: "查看订阅状态",
			Usage:       "/订阅状态",
			Permission:  "everyone",
		}, {
			Name:        "立即检查订阅",
			Description: "立即检查当前会话或全部订阅",
			Usage:       "/立即检查订阅 [当前|全部]",
			Permission:  "super_admin",
		}}, config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"#", "*"}}}}),
		"help": buildBuiltinHelp(&plugins.HelpView{
			Groups: []plugins.HelpGroupView{{
				Title: "订阅操作",
				Items: []plugins.HelpItemView{{
					Title:       "订阅状态",
					Description: "查看启用状态。",
					Command:     "订阅状态",
					Usage:       "/订阅状态",
					Permission:  "everyone",
				}},
			}, {
				Title: "维护与预览",
				Items: []plugins.HelpItemView{{
					Title:       "立即检查订阅",
					Description: "立即检查当前会话或全部订阅。",
					Command:     "立即检查订阅",
					Usage:       "/立即检查订阅 [当前|全部]",
					Permission:  "super_admin",
				}},
			}},
		}, nil, config.Config{}),
	}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Prefixes: []string{"#", "*"},
		}},
	})

	groups, ok := data["groups"].([]map[string]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("unexpected plugin menu groups: %#v", data["groups"])
	}
	if got := groups[0]["title"]; got != "订阅操作" {
		t.Fatalf("first group title = %#v, want 订阅操作", got)
	}
	if got := groups[1]["title"]; got != "维护与预览" {
		t.Fatalf("second group title = %#v, want 维护与预览", got)
	}
	for _, group := range groups {
		if group["title"] == "命令" {
			t.Fatalf("covered commands should not render a duplicate command group: %#v", groups)
		}
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			if got := item["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
				t.Fatalf("command item prefixes = %#v, want [# *]", got)
			}
		}
	}
	items, _ := groups[1]["items"].([]map[string]any)
	if got := items[0]["usage_args"]; got != "[当前|全部]" {
		t.Fatalf("usage_args = %#v, want [当前|全部]", got)
	}
}

func TestVisibleBuiltinHelpInheritsCommandPermission(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"#"}}}}
	commands := []plugins.CommandView{{
		Name:        "订阅状态",
		Description: "查看订阅状态",
		Permission:  "everyone",
	}, {
		Name:        "立即检查订阅",
		Description: "立即检查当前会话或全部订阅",
		Permission:  "super_admin",
	}}
	help := &plugins.HelpView{
		Groups: []plugins.HelpGroupView{{
			Title: "订阅操作",
			Items: []plugins.HelpItemView{{
				Title:       "订阅状态",
				Description: "查看启用状态。",
				Command:     "订阅状态",
				Usage:       "/订阅状态",
			}, {
				Title:       "立即检查订阅",
				Description: "立即检查当前会话或全部订阅。",
				Command:     "立即检查订阅",
				Usage:       "/立即检查订阅 [当前|全部]",
			}, {
				Title:       "动态有效时间",
				Description: "动态只在设置的时间窗口内推送。",
				Permission:  "super_admin",
			}},
		}},
	}

	memberEvent := runtimeEventFromAdapter(adapterintake.NormalizedEvent{ActorRole: "member"})
	memberCommands := visibleBuiltinCommands(commands, cfg, memberEvent)
	memberHelp := visibleBuiltinHelp(help, commands, memberCommands, cfg, memberEvent)
	memberMenu := buildBuiltinHelp(memberHelp, commands, cfg)
	memberGroups, ok := memberMenu["groups"].([]map[string]any)
	if !ok || len(memberGroups) != 1 {
		t.Fatalf("unexpected member help groups: %#v", memberMenu["groups"])
	}
	memberItems, ok := memberGroups[0]["items"].([]map[string]any)
	if !ok || len(memberItems) != 1 {
		t.Fatalf("unexpected member help items: %#v", memberGroups[0]["items"])
	}
	if got := memberItems[0]["name"]; got != "订阅状态" {
		t.Fatalf("member help item name = %#v, want 订阅状态", got)
	}
	if got := memberItems[0]["permission"]; got != "everyone" {
		t.Fatalf("member command permission = %#v, want everyone", got)
	}
	if got := memberItems[0]["permission_label"]; got != "所有人" {
		t.Fatalf("member command permission_label = %#v, want 所有人", got)
	}

	superEvent := runtimeEventFromAdapter(adapterintake.NormalizedEvent{
		SenderID:  "10001",
		ActorRole: "member",
	})
	superCfg := cfg
	superCfg.Admin.SuperAdmins = []string{"10001"}
	superCommands := visibleBuiltinCommands(commands, superCfg, superEvent)
	superHelp := visibleBuiltinHelp(help, commands, superCommands, superCfg, superEvent)
	superMenu := buildBuiltinHelp(superHelp, commands, superCfg)
	superGroups, ok := superMenu["groups"].([]map[string]any)
	if !ok || len(superGroups) != 1 {
		t.Fatalf("unexpected super admin help groups: %#v", superMenu["groups"])
	}
	superItems, ok := superGroups[0]["items"].([]map[string]any)
	if !ok || len(superItems) != 3 {
		t.Fatalf("unexpected super admin help items: %#v", superGroups[0]["items"])
	}
	if got := superItems[1]["permission"]; got != "super_admin" {
		t.Fatalf("super admin command permission = %#v, want super_admin", got)
	}
	if got := superItems[1]["permission_label"]; got != "超级管理员" {
		t.Fatalf("super admin command permission_label = %#v, want 超级管理员", got)
	}
	if got := superItems[2]["permission"]; got != "super_admin" {
		t.Fatalf("explicit help permission = %#v, want super_admin", got)
	}
}
