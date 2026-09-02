package catalog

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestProjectCommandsUsesDefaultDynamicSetting(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "fortune", DisplayName: "今日运势", TriggerType: "setting", SettingsKey: "trigger_commands",
			Description: "查看今日运势", Usage: "/我的运势 [日期]", Permission: "everyone",
		}},
		DefaultConfig: map[string]any{
			"trigger_commands": []any{" 我的运势 ", "今日运势", "我的运势"},
		},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	got := commands[0]
	if got.Name != "我的运势" || !reflect.DeepEqual(got.Aliases, []string{"今日运势"}) {
		t.Fatalf("unexpected dynamic command tokens: %#v", got)
	}
	if got.Usage != "/我的运势 [日期]" || got.TriggerType != "setting" || got.ID != "fortune" || got.Permission != "everyone" {
		t.Fatalf("unexpected dynamic command metadata: %#v", got)
	}
}

func TestProjectCommandsUsesPersistedDynamicSetting(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "fortune", DisplayName: "今日运势", TriggerType: "setting", SettingsKey: "trigger_commands",
			Description: "查看今日运势",
		}},
		DefaultConfig: map[string]any{
			"trigger_commands": []any{"我的运势"},
		},
	}

	commands := ProjectCommands(snapshot, map[string]any{
		"trigger_commands": []string{"今日签", "每日签"},
	})
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	if commands[0].Name != "今日签" || !reflect.DeepEqual(commands[0].Aliases, []string{"每日签"}) {
		t.Fatalf("unexpected persisted dynamic command: %#v", commands[0])
	}
}

func TestProjectCommandsKeepsExplicitEmptyDynamicSetting(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "fortune", DisplayName: "今日运势", TriggerType: "setting", SettingsKey: "trigger_commands",
			Description: "查看今日运势",
		}},
		DefaultConfig: map[string]any{
			"trigger_commands": []any{"我的运势"},
		},
	}

	commands := ProjectCommands(snapshot, map[string]any{
		"trigger_commands": []any{},
	})
	if len(commands) != 0 {
		t.Fatalf("commands = %#v, want empty", commands)
	}
}

func TestProjectCommandsIgnoresWhitespaceDynamicTokens(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "fortune", DisplayName: "今日运势", TriggerType: "setting", SettingsKey: "trigger_commands",
			Description: "查看今日运势",
		}},
		DefaultConfig: map[string]any{
			"trigger_commands": []any{"我的 运势", "今日运势", "今日 运势"},
		},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 1 || commands[0].Name != "今日运势" {
		t.Fatalf("commands = %#v, want only 今日运势", commands)
	}
}

func TestProjectCommandsMarksManifestCommands(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "status", DisplayName: "订阅状态", TriggerType: "exact",
			TriggerNames: []string{" 订阅状态 ", "状态？", "订阅📡", "状态？", "订阅 状态"},
		}},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	if commands[0].Name != "订阅状态" || !reflect.DeepEqual(commands[0].Aliases, []string{"状态？", "订阅📡"}) || commands[0].TriggerType != "exact" {
		t.Fatalf("unexpected manifest command projection: %#v", commands[0])
	}
}

func TestProjectCommandsProjectsCommandPatterns(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "character-guide", DisplayName: " 角色攻略 ", TriggerType: "pattern", MatchPattern: "^(.+?)攻略$",
			Description: "按角色名查询攻略图",
			Usage:       "*<角色名>攻略",
			Permission:  "everyone",
		}},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	got := commands[0]
	if got.Name != "角色攻略" || got.MatchPattern != "^(.+?)攻略$" || got.TriggerType != "pattern" {
		t.Fatalf("unexpected command pattern projection: %#v", got)
	}
	if got.ID != "character-guide" || got.Usage != "*<角色名>攻略" || got.Permission != "everyone" {
		t.Fatalf("unexpected command pattern metadata: %#v", got)
	}
}

func TestProjectCommandsIgnoresInvalidCommandPatterns(t *testing.T) {
	snapshot := plugins.Snapshot{
		ManifestCommands: []plugins.Command{{
			ID: "bad-pattern", DisplayName: "角色攻略", TriggerType: "pattern", MatchPattern: "[",
		}},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 0 {
		t.Fatalf("commands = %#v, want empty", commands)
	}
}
