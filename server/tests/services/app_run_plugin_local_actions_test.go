package services

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func boolPointer(value bool) *bool {
	return &value
}

func TestExecutePluginPrivateKVWithoutDeclaredPermission(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := pluginstore.NewKVSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewKVSQLiteRepository: %v", err)
	}
	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubPermissionView{permissions: map[string][]stubPermission{}},
		nil,
		nil,
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_1", plugins.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	if err != nil || result["exists"] != false {
		t.Fatalf("implicit KV access result = %#v, err = %v", result, err)
	}
}

func TestExecutePluginListUsesDeclaredPermission(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "raylea.echo",
			Name:              "Echo",
			SourceRoot:        "plugins/installed",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Permissions:       map[string]plugins.PermissionGrant{"plugin.list": {}},
			Commands: []plugins.Command{{
				ID:           "echo",
				Name:         "echo",
				DisplayName:  "echo",
				TriggerType:  "exact",
				TriggerNames: []string{"echo"},
				Description:  "复读内容",
				Usage:        "/echo <内容>",
			}},
		},
		{
			PluginID:          "raylea.tools",
			Name:              "Tools",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				ID:           "tool",
				Name:         "tool",
				DisplayName:  "tool",
				TriggerType:  "exact",
				TriggerNames: []string{"tool"},
				Description:  "工具命令",
				Usage:        "/tool",
			}},
		},
	})
	application.setTestLocalActions(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "raylea.echo", "req_local_plugin_list_1", plugins.Action{
		Kind: "plugin.list",
	})
	if err != nil {
		t.Fatalf("plugin.list failed: %v", err)
	}

	items, ok := result["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected plugin list items: %#v", result["items"])
	}
	if items[0]["id"] != "raylea.echo" || items[1]["id"] != "raylea.tools" {
		t.Fatalf("unexpected plugin order: %#v", items)
	}
	echoCommands, ok := items[0]["commands"].([]map[string]any)
	if !ok || len(echoCommands) != 1 {
		t.Fatalf("unexpected echo commands: %#v", items[0]["commands"])
	}
	trigger, _ := echoCommands[0]["trigger"].(map[string]any)
	if echoCommands[0]["id"] != "echo" || echoCommands[0]["name"] != "echo" || trigger["type"] != "exact" {
		t.Fatalf("unexpected echo command projection: %#v", echoCommands[0])
	}
}

func TestExecutePluginListCallerVisibilityFiltersCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    config.Config
		event     chatevent.Event
		wantNames []string
	}{
		{
			name: "member sees everyone commands",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			event:     pluginListCallerEvent("1001", "member", "group"),
			wantNames: []string{"public", "defaulted"},
		},
		{
			name: "admin sees group admin commands",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			event:     pluginListCallerEvent("1002", "admin", "group"),
			wantNames: []string{"public", "admin", "defaulted"},
		},
		{
			name: "owner sees group admin commands",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			event:     pluginListCallerEvent("1003", "owner", "group"),
			wantNames: []string{"public", "admin", "defaulted"},
		},
		{
			name: "super admin sees all commands",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			event:     pluginListCallerEvent("9001", "member", "private"),
			wantNames: []string{"public", "admin", "super", "defaulted"},
		},
		{
			name: "default permission applies to undeclared commands",
			config: config.Config{
				Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
			},
			event:     pluginListCallerEvent("1004", "member", "group"),
			wantNames: []string{"public"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			application := newPluginListVisibilityTestApp(tc.config)
			result, err := application.executeLocalActionForEvent(context.Background(), "raylea.echo", "req_local_plugin_list_visibility", plugins.Action{
				Kind:                 "plugin.list",
				PluginListVisibility: "caller",
			}, tc.event)
			if err != nil {
				t.Fatalf("plugin.list failed: %v", err)
			}

			gotNames := pluginListCommandNamesForPlugin(t, result, "raylea.tools")
			if strings.Join(gotNames, ",") != strings.Join(tc.wantNames, ",") {
				t.Fatalf("visible commands = %#v, want %#v", gotNames, tc.wantNames)
			}
		})
	}
}

func TestExecutePluginListCallerVisibilityFiltersHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         config.Config
		event          chatevent.Event
		wantHelpTitles []string
	}{
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "member sees public help",
			event:          pluginListCallerEvent("1001", "member", "group"),
			wantHelpTitles: []string{"Tools"},
		},
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "admin sees group admin help",
			event:          pluginListCallerEvent("1002", "admin", "group"),
			wantHelpTitles: []string{"Tools"},
		},
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "super admin sees all help",
			event:          pluginListCallerEvent("9001", "member", "private"),
			wantHelpTitles: []string{"Tools"},
		},
		{
			name: "independent help without permission defaults to everyone",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
			},
			event:          pluginListCallerEvent("1001", "member", "group"),
			wantHelpTitles: []string{"Tools"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			application := newPluginListVisibilityTestApp(tc.config)
			result, err := application.executeLocalActionForEvent(context.Background(), "raylea.echo", "req_local_plugin_list_help_visibility", plugins.Action{
				Kind:                 "plugin.list",
				PluginListVisibility: "caller",
			}, tc.event)
			if err != nil {
				t.Fatalf("plugin.list failed: %v", err)
			}

			gotTitles := pluginListHelpTitlesForPlugin(t, result, "raylea.tools")
			if strings.Join(gotTitles, ",") != strings.Join(tc.wantHelpTitles, ",") {
				t.Fatalf("visible help titles = %#v, want %#v", gotTitles, tc.wantHelpTitles)
			}
		})
	}
}

func newPluginListVisibilityTestApp(cfg config.Config) *serviceHarness {
	application := newTestAppState(cfg, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "raylea.echo",
			Name:              "Echo",
			SourceRoot:        "plugins/installed",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Permissions:       map[string]plugins.PermissionGrant{"plugin.list": {}},
		},
		{
			PluginID:          "raylea.tools",
			Name:              "Tools",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{
				{ID: "public", Name: "public", DisplayName: "public", TriggerType: "exact", TriggerNames: []string{"public"}, Permission: "everyone"},
				{ID: "admin", Name: "admin", DisplayName: "admin", TriggerType: "exact", TriggerNames: []string{"admin"}, Permission: "group_admin"},
				{ID: "super", Name: "super", DisplayName: "super", TriggerType: "exact", TriggerNames: []string{"super"}, Permission: "super_admin"},
				{ID: "defaulted", Name: "defaulted", DisplayName: "defaulted", TriggerType: "exact", TriggerNames: []string{"defaulted"}},
			},
			Help: &plugins.Help{
				Title:   "Tools",
				Summary: "工具说明",
			},
			CommandGroups: []plugins.CommandGroup{{ID: "tools", Title: "工具", Commands: []string{"public", "admin", "super", "defaulted"}}},
		},
	})
	application.setTestLocalActions(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	return application
}

func pluginListCallerEvent(actorID, actorRole, targetType string) chatevent.Event {
	event := chatevent.Event{
		EventID:        "event-help-visibility",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message." + targetType,
		Timestamp:      time.Now().Unix(),
		Actor: &chatevent.Actor{
			ID:   actorID,
			Role: actorRole,
		},
		Target: &chatevent.Target{
			Type: targetType,
			ID:   actorID,
		},
	}
	if targetType == "group" {
		event.Target.ID = "2001"
	}
	return event
}

func pluginListCommandNamesForPlugin(t *testing.T, result map[string]any, pluginID string) []string {
	t.Helper()

	items, ok := result["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected plugin list items: %#v", result["items"])
	}
	for _, item := range items {
		if item["id"] != pluginID {
			continue
		}
		commands, ok := item["commands"].([]map[string]any)
		if !ok {
			t.Fatalf("unexpected commands for %s: %#v", pluginID, item["commands"])
		}
		names := make([]string, 0, len(commands))
		for _, command := range commands {
			name, _ := command["name"].(string)
			names = append(names, name)
		}
		return names
	}
	t.Fatalf("plugin %s not found in result: %#v", pluginID, result)
	return nil
}

func pluginListHelpTitlesForPlugin(t *testing.T, result map[string]any, pluginID string) []string {
	t.Helper()

	items, ok := result["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected plugin list items: %#v", result["items"])
	}
	for _, item := range items {
		if item["id"] != pluginID {
			continue
		}
		help, ok := item["help"].(map[string]any)
		if !ok {
			t.Fatalf("unexpected help for %s: %#v", pluginID, item["help"])
		}
		title, _ := help["title"].(string)
		return []string{title}
	}
	t.Fatalf("plugin %s not found in result: %#v", pluginID, result)
	return nil
}

func TestExecuteSecretReadReturnsPluginScopedValue(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("secrets.NewSQLiteStore: %v", err)
	}
	sealedPrimary, err := secrets.SealString(context.Background(), secretStore, "SESSDATA=fixture")
	if err != nil {
		t.Fatalf("secrets.SealString primary: %v", err)
	}
	if err := secretStore.Set(context.Background(), "plugin:subscription-hub:secret:bili_token_primary", sealedPrimary); err != nil {
		t.Fatalf("secretStore.Set: %v", err)
	}
	sealedOther, err := secrets.SealString(context.Background(), secretStore, "SESSDATA=other")
	if err != nil {
		t.Fatalf("secrets.SealString other: %v", err)
	}
	if err := secretStore.Set(context.Background(), "plugin:other-plugin:secret:bili_token_primary", sealedOther); err != nil {
		t.Fatalf("secretStore.Set other: %v", err)
	}

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "subscription-hub",
		Valid:             true,
		RegistrationState: "installed",
		Permissions:       map[string]plugins.PermissionGrant{"secret.read": {}},
	}})
	application.platform.Secrets = secretStore
	application.setTestLocalActions(
		&stubPermissionView{permissions: map[string][]stubPermission{
			"subscription-hub": {{
				PluginID:   "subscription-hub",
				Permission: "secret.read",
			}},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_1", plugins.Action{
		Kind:      "secret.read",
		SecretKey: "bili_token_primary",
	})
	if err != nil {
		t.Fatalf("secret.read failed: %v", err)
	}
	if result["exists"] != true || result["value"] != "SESSDATA=fixture" {
		t.Fatalf("unexpected secret.read result: %#v", result)
	}

	missing, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_2", plugins.Action{
		Kind:      "secret.read",
		SecretKey: "missing",
	})
	if err != nil {
		t.Fatalf("secret.read missing failed: %v", err)
	}
	if missing["exists"] != false {
		t.Fatalf("unexpected missing result: %#v", missing)
	}
}

func TestExecuteSecretReadRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubPermissionView{permissions: map[string][]stubPermission{
			"subscription-hub": {{
				PluginID:   "subscription-hub",
				Permission: "secret.read",
			}},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_invalid", plugins.Action{
		Kind:      "secret.read",
		SecretKey: "Bad Key",
	})
	assertRuntimeErrorCode(t, err, "plugin.protocol_violation")
}
