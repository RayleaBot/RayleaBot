package plugins

import (
	"reflect"
	"testing"
)

func TestProjectCommandsUsesDefaultDynamicSetting(t *testing.T) {
	snapshot := Snapshot{
		DynamicCommands: []DynamicCommandDecl{{
			ID:          "fortune",
			SettingsKey: "trigger_commands",
			Description: "查看今日运势",
			UsageArgs:   "[日期]",
			Permission:  "everyone",
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
	if got.Usage != "我的运势 [日期]" || got.CommandSource != CommandSourceDynamic || got.DeclarationID != "fortune" || got.Permission != "everyone" {
		t.Fatalf("unexpected dynamic command metadata: %#v", got)
	}
}

func TestProjectCommandsUsesPersistedDynamicSetting(t *testing.T) {
	snapshot := Snapshot{
		DynamicCommands: []DynamicCommandDecl{{
			ID:          "fortune",
			SettingsKey: "trigger_commands",
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
	snapshot := Snapshot{
		DynamicCommands: []DynamicCommandDecl{{
			ID:          "fortune",
			SettingsKey: "trigger_commands",
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
	snapshot := Snapshot{
		DynamicCommands: []DynamicCommandDecl{{
			ID:          "fortune",
			SettingsKey: "trigger_commands",
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
	snapshot := Snapshot{
		ManifestCommands: []Command{{
			Name:    " help ",
			Aliases: []string{"commands", "help"},
		}},
	}

	commands := ProjectCommands(snapshot, nil)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	if commands[0].Name != "help" || !reflect.DeepEqual(commands[0].Aliases, []string{"commands", "help"}) || commands[0].CommandSource != CommandSourceManifest {
		t.Fatalf("unexpected manifest command projection: %#v", commands[0])
	}
}
