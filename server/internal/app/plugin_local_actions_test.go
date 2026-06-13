package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func boolPointer(value bool) *bool {
	return &value
}

func TestExecuteLocalActionRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

	_, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_1", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecutePluginListUsesBuiltinAutoGrant(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:            "raylea.echo",
			Name:                "Echo",
			SourceRoot:          "plugins/builtin",
			Valid:               true,
			RegistrationState:   "installed",
			DesiredState:        "enabled",
			RuntimeState:        "running",
			RequiredPermissions: []string{"plugin.list"},
			Commands: []plugins.Command{{
				Name:          "echo",
				Description:   "复读内容",
				Usage:         "/echo <内容>",
				CommandSource: plugins.CommandSourceManifest,
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
				Name:          "tool",
				Description:   "工具命令",
				Usage:         "/tool",
				CommandSource: plugins.CommandSourceManifest,
			}},
		},
	})
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

	result, err := application.executeLocalAction(context.Background(), "raylea.echo", "req_local_plugin_list_1", runtime.Action{
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
	if echoCommands[0]["name"] != "echo" || echoCommands[0]["command_source"] != "manifest" {
		t.Fatalf("unexpected echo command projection: %#v", echoCommands[0])
	}
}

func TestExecutePluginListCallerVisibilityFiltersCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    config.Config
		event     runtime.Event
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
			result, err := application.executeLocalActionForEvent(context.Background(), "raylea.echo", "req_local_plugin_list_visibility", runtime.Action{
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
		event          runtime.Event
		wantHelpTitles []string
	}{
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "member sees public help",
			event:          pluginListCallerEvent("1001", "member", "group"),
			wantHelpTitles: []string{"公开说明", "独立公开说明"},
		},
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "admin sees group admin help",
			event:          pluginListCallerEvent("1002", "admin", "group"),
			wantHelpTitles: []string{"公开说明", "管理说明", "独立公开说明", "独立管理说明"},
		},
		{
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "everyone"},
			},
			name:           "super admin sees all help",
			event:          pluginListCallerEvent("9001", "member", "private"),
			wantHelpTitles: []string{"公开说明", "管理说明", "超管说明", "独立公开说明", "独立管理说明", "独立超管说明"},
		},
		{
			name: "independent help without permission defaults to everyone",
			config: config.Config{
				Admin:      config.AdminConfig{SuperAdmins: []string{"9001"}},
				Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
			},
			event:          pluginListCallerEvent("1001", "member", "group"),
			wantHelpTitles: []string{"公开说明", "独立公开说明"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			application := newPluginListVisibilityTestApp(tc.config)
			result, err := application.executeLocalActionForEvent(context.Background(), "raylea.echo", "req_local_plugin_list_help_visibility", runtime.Action{
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

func newPluginListVisibilityTestApp(cfg config.Config) *App {
	application := newTestAppState(cfg, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:            "raylea.echo",
			Name:                "Echo",
			SourceRoot:          "plugins/builtin",
			Valid:               true,
			RegistrationState:   "installed",
			DesiredState:        "enabled",
			RuntimeState:        "running",
			RequiredPermissions: []string{"plugin.list"},
		},
		{
			PluginID:          "raylea.tools",
			Name:              "Tools",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{
				{Name: "public", Permission: "everyone", CommandSource: plugins.CommandSourceManifest},
				{Name: "admin", Permission: "group_admin", CommandSource: plugins.CommandSourceManifest},
				{Name: "super", Permission: "super_admin", CommandSource: plugins.CommandSourceManifest},
				{Name: "defaulted", CommandSource: plugins.CommandSourceManifest},
			},
			Help: &plugins.Help{
				Title:   "Tools",
				Summary: "工具说明",
				Groups: []plugins.HelpGroup{{
					Title: "权限说明",
					Items: []plugins.HelpItem{
						{Title: "公开说明", Command: "public"},
						{Title: "管理说明", Command: "admin"},
						{Title: "超管说明", Command: "super"},
						{Title: "未知指令说明", Command: "missing"},
						{Title: "独立公开说明"},
						{Title: "独立管理说明", Permission: "group_admin"},
						{Title: "独立超管说明", Permission: "super_admin"},
					},
				}},
			},
		},
	})
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

func pluginListCallerEvent(actorID, actorRole, targetType string) runtime.Event {
	event := runtime.Event{
		EventID:        "event-help-visibility",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message." + targetType,
		Timestamp:      time.Now().Unix(),
		Actor: &runtime.EventActor{
			ID:   actorID,
			Role: actorRole,
		},
		Target: &runtime.EventTarget{
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
		groups, ok := help["groups"].([]map[string]any)
		if !ok {
			t.Fatalf("unexpected help groups for %s: %#v", pluginID, help["groups"])
		}
		var titles []string
		for _, group := range groups {
			entries, ok := group["items"].([]map[string]any)
			if !ok {
				t.Fatalf("unexpected help items for %s: %#v", pluginID, group["items"])
			}
			for _, entry := range entries {
				titles = append(titles, entry["title"].(string))
			}
		}
		return titles
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
	application.pluginStack.Plugins = plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:            "subscription-hub",
		Valid:               true,
		RegistrationState:   "installed",
		RequiredPermissions: []string{"secret.read"},
	}})
	application.platform.Secrets = secretStore
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{
			"subscription-hub": {{
				PluginID:   "subscription-hub",
				Capability: "secret.read",
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

	result, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_1", runtime.Action{
		Kind:      "secret.read",
		SecretKey: "bili_token_primary",
	})
	if err != nil {
		t.Fatalf("secret.read failed: %v", err)
	}
	if result["exists"] != true || result["value"] != "SESSDATA=fixture" {
		t.Fatalf("unexpected secret.read result: %#v", result)
	}

	missing, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_2", runtime.Action{
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
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{
			"subscription-hub": {{
				PluginID:   "subscription-hub",
				Capability: "secret.read",
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

	_, err := application.executeLocalAction(context.Background(), "subscription-hub", "req_local_secret_invalid", runtime.Action{
		Kind:      "secret.read",
		SecretKey: "Bad Key",
	})
	assertRuntimeErrorCode(t, err, "plugin.protocol_violation")
}

func TestExecuteLoggerWriteAppliesRateLimit(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"logger.write"},
		},
		Log: config.LogConfig{
			RateLimitPerPlugin: "1/1h",
		},
	}, slog.New(slog.NewJSONHandler(buffer, nil)))
	application.state.redactText = func(text string) string {
		return text
	}
	application.setTestLocalActions(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		newPluginLogLimiter(config.Config{Log: config.LogConfig{RateLimitPerPlugin: "1/1h"}}),
		nil,
	)

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_2", runtime.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "first log",
	}); err != nil {
		t.Fatalf("first logger.write failed: %v", err)
	}

	_, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_3", runtime.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "second log",
	})
	assertRuntimeErrorCode(t, err, "platform.rate_limited")
}

func TestExecuteStorageKVRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginkv.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := newTestAppState(config.Config{
		Storage: config.StorageConfig{
			KVValueMaxBytes: 1024,
			KVTotalLimitMB:  1,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"notice-logger": {{
					PluginID:   "notice-logger",
					Capability: "storage.kv",
				}},
			},
		},
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

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_4", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "set",
		StorageKey:       "notice:last_join",
		StorageValue: map[string]any{
			"user_id": "3001",
			"count":   2,
		},
	}); err != nil {
		t.Fatalf("storage set failed: %v", err)
	}

	getResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_5", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage get failed: %v", err)
	}
	if exists, _ := getResult["exists"].(bool); !exists {
		t.Fatalf("expected get exists=true, got %#v", getResult)
	}

	listResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_6", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "list",
		StoragePrefix:    "notice:",
	})
	if err != nil {
		t.Fatalf("storage list failed: %v", err)
	}
	keys, _ := listResult["keys"].([]string)
	if len(keys) != 1 || keys[0] != "notice:last_join" {
		t.Fatalf("unexpected list keys: %#v", listResult["keys"])
	}

	deleteResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_7", runtime.Action{
		Kind:             "storage.kv",
		StorageOperation: "delete",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage delete failed: %v", err)
	}
	if deleted, _ := deleteResult["deleted"].(bool); !deleted {
		t.Fatalf("expected delete deleted=true, got %#v", deleteResult)
	}
}

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

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"config.read", "config.write"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := repo.SeedDefaults(context.Background(), "weather", map[string]any{
		"default_city": "北京",
		"unit":         "celsius",
	}); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	readResult, err := application.executeLocalAction(context.Background(), "weather", "req_config_1", runtime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit", "missing"},
	})
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

	writeResult, err := application.executeLocalAction(context.Background(), "weather", "req_config_2", runtime.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
			"unit":         "fahrenheit",
		},
	})
	if err != nil {
		t.Fatalf("config.write failed: %v", err)
	}
	changedKeys, _ := writeResult["changed_keys"].([]string)
	if len(changedKeys) != 2 || changedKeys[0] != "default_city" || changedKeys[1] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", writeResult["changed_keys"])
	}

	readResult, err = application.executeLocalAction(context.Background(), "weather", "req_config_3", runtime.Action{
		Kind:       "config.read",
		ConfigKeys: []string{"default_city", "unit"},
	})
	if err != nil {
		t.Fatalf("config.read second call failed: %v", err)
	}
	values, _ = readResult["values"].(map[string]any)
	if values["default_city"] != "上海" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated config values: %#v", values)
	}
}

func TestExecuteConfigWriteDispatchesConfigChanged(t *testing.T) {
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

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"config.write"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		repo,
		nil,
		nil,
		nil,
		dispatcher,
		nil,
		nil,
		nil,
		nil,
	)
	fakeRuntime := &capturingRuntime{events: make(chan runtime.Event, 1)}
	application.pluginStack.Dispatcher.Register("weather", fakeRuntime, []string{"config.changed"}, nil, 1)

	if _, err := application.executeLocalAction(context.Background(), "weather", "req_config_changed", runtime.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
		},
	}); err != nil {
		t.Fatalf("config.write failed: %v", err)
	}

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "config.changed" {
			t.Fatalf("unexpected config.changed event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected config.changed event")
	}
}

func TestExecuteGovernanceActionsRejectMissingCapability(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.pluginStack.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.pluginStack.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

	_, err = application.executeLocalAction(context.Background(), "governance-helper", "req_governance_unauthorized", runtime.Action{
		Kind: "governance.blacklist.read",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteGovernanceActionsRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{
				"governance.blacklist.read",
				"governance.blacklist.write",
				"governance.whitelist.read",
				"governance.whitelist.write",
				"governance.command_policy.read",
			},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.pluginStack.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.pluginStack.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.pluginStack.Plugins = plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		Commands: []plugins.Command{
			{Name: "forecast", Permission: "group_admin", Aliases: []string{"fc"}, CommandSource: plugins.CommandSourceManifest},
			{Name: "current", CommandSource: plugins.CommandSourceManifest},
		},
	}})
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

	blacklistWrite, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_upsert", runtime.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	})
	if err != nil {
		t.Fatalf("governance.blacklist.write upsert failed: %v", err)
	}
	if blacklistWrite["entry_type"] != "user" || blacklistWrite["target_id"] != "1001" {
		t.Fatalf("unexpected blacklist write result: %#v", blacklistWrite)
	}

	blacklistRead, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_read", runtime.Action{
		Kind: "governance.blacklist.read",
	})
	if err != nil {
		t.Fatalf("governance.blacklist.read failed: %v", err)
	}
	userEntries, _ := blacklistRead["user_entries"].([]governance.EntryResponse)
	if len(userEntries) != 1 || userEntries[0].TargetID != "1001" {
		t.Fatalf("unexpected blacklist snapshot: %#v", blacklistRead)
	}

	whitelistToggle, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_enabled", runtime.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "set_enabled",
		GovernanceEnabled:   boolPointer(true),
	})
	if err != nil {
		t.Fatalf("governance.whitelist.write set_enabled failed: %v", err)
	}
	if whitelistToggle["enabled"] != true {
		t.Fatalf("unexpected whitelist toggle result: %#v", whitelistToggle)
	}

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_upsert", runtime.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "group",
		GovernanceTargetID:  "2001",
		GovernanceReason:    "approved",
	}); err != nil {
		t.Fatalf("governance.whitelist.write upsert failed: %v", err)
	}

	whitelistRead, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_read", runtime.Action{
		Kind: "governance.whitelist.read",
	})
	if err != nil {
		t.Fatalf("governance.whitelist.read failed: %v", err)
	}
	groupEntries, _ := whitelistRead["group_entries"].([]governance.EntryResponse)
	if whitelistRead["enabled"] != true || len(groupEntries) != 1 || groupEntries[0].TargetID != "2001" {
		t.Fatalf("unexpected whitelist snapshot: %#v", whitelistRead)
	}

	commandPolicy, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_command_policy", runtime.Action{
		Kind: "governance.command_policy.read",
	})
	if err != nil {
		t.Fatalf("governance.command_policy.read failed: %v", err)
	}
	commands, _ := commandPolicy["commands"].([]governance.CommandPolicyEntryResponse)
	if commandPolicy["default_level"] != "everyone" || len(commands) != 2 {
		t.Fatalf("unexpected command policy: %#v", commandPolicy)
	}
	for _, command := range commands {
		if command.CommandSource != "manifest" {
			t.Fatalf("unexpected command source in policy: %#v", command)
		}
	}

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_delete", runtime.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "delete",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
	}); err != nil {
		t.Fatalf("governance.blacklist.write delete failed: %v", err)
	}
}

func TestExecuteGovernanceWritePublishesGovernanceChanged(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"governance.blacklist.write"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.pluginStack.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.pluginStack.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
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

	events, unsubscribe := application.services.governanceEvents.subscribeGovernanceEvents(1)
	defer unsubscribe()

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_publish", runtime.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	}); err != nil {
		t.Fatalf("governance.blacklist.write upsert failed: %v", err)
	}

	select {
	case frame := <-events:
		data, ok := frame.Data.(genericManagementEventPayload)
		if !ok || data.EventType != "governance.changed" {
			t.Fatalf("unexpected governance event: %#v", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected governance.changed event")
	}
}

func TestExecuteSchedulerCreateUpsertDoesNotWriteManagementLog(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"scheduler.create"},
		},
	}, slog.New(slog.NewTextHandler(buffer, nil)))
	application.pluginStack.Plugins = plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "天气插件",
		Valid:             true,
		RegistrationState: "installed",
	}})
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		engine,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	first, err := application.executeLocalAction(context.Background(), "weather", "req_sched_1", runtime.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "每日早报",
		SchedulerCron:      "0 8 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report",
		},
	})
	if err != nil {
		t.Fatalf("first scheduler.create failed: %v", err)
	}
	if first["task_id"] != "daily_report" {
		t.Fatalf("unexpected task_id: %#v", first["task_id"])
	}
	if _, ok := first["next_run"].(string); !ok {
		t.Fatalf("expected next_run string, got %#v", first["next_run"])
	}

	second, err := application.executeLocalAction(context.Background(), "weather", "req_sched_2", runtime.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "新版早报",
		SchedulerCron:      "30 9 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report_v2",
		},
	})
	if err != nil {
		t.Fatalf("second scheduler.create failed: %v", err)
	}
	if second["task_id"] != "daily_report" {
		t.Fatalf("unexpected second task_id: %#v", second["task_id"])
	}

	jobs := engine.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].JobID != "daily_report" || jobs[0].CronExpr != "30 9 * * *" {
		t.Fatalf("unexpected upserted job: %#v", jobs[0])
	}
	if jobs[0].LogLabel != "新版早报" {
		t.Fatalf("LogLabel = %q, want 新版早报", jobs[0].LogLabel)
	}
	if logs := buffer.String(); strings.Contains(logs, "定时任务已注册") {
		t.Fatalf("scheduler registration should not write management log:\n%s", logs)
	}
}

func TestExecuteExposeWebhookRegistersGateway(t *testing.T) {
	t.Parallel()

	grantRepo := &stubLifecycleGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"repo-watcher": {{
				PluginID:   "repo-watcher",
				Capability: "event.expose_webhook",
				ScopeJSON:  `{"webhooks":[{"route":"github","auth_strategy":"hmac_sha256","header":"X-Hub-Signature-256","secret_ref":"webhook.github.secret","source_ips":["192.0.2.0/24"]}]}`,
			}},
		},
	}
	application := newTestAppState(config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"event.expose_webhook"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	registry := newPluginWebhookRegistry()
	application.setTestLocalActions(grantRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	application.setTestWebhookService(nil, nil, nil, registry)

	result, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_1", runtime.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
		WebhookReplayProtection: &runtime.WebhookReplayProtection{
			TimestampHeader:  "X-Raylea-Timestamp",
			EventIDHeader:    "X-Raylea-Event-Id",
			ToleranceSeconds: 300,
			Enforce:          true,
		},
	})
	if err != nil {
		t.Fatalf("event.expose_webhook failed: %v", err)
	}
	if result["route"] != "github" {
		t.Fatalf("unexpected route result: %#v", result)
	}
	urlValue, _ := result["url"].(string)
	if urlValue != "http://127.0.0.1:8080/api/webhooks/repo-watcher/github" {
		t.Fatalf("unexpected webhook url: %#v", urlValue)
	}

	registration, ok := application.pluginStack.webhooks.Get("repo-watcher", "github")
	if !ok {
		t.Fatal("expected webhook registration to be stored")
	}
	if registration.AuthStrategy != "hmac_sha256" || registration.SecretRef != "webhook.github.secret" {
		t.Fatalf("unexpected webhook registration: %#v", registration)
	}
	if len(registration.SourceIPs) != 1 || registration.SourceIPs[0] != "192.0.2.0/24" {
		t.Fatalf("unexpected webhook source IPs: %#v", registration.SourceIPs)
	}
}

func TestExecuteStorageFileRoundTrip(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Storage: config.StorageConfig{
			FileMaxBytes:    1024,
			PluginWorkDirMB: 1,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "storage.file",
					ScopeJSON:  `{"storage_roots":["plugin_data"]}`,
				}},
			},
		},
		nil,
		pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_1", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
		StorageContent:   []byte("hello file"),
	}); err != nil {
		t.Fatalf("storage.file write failed: %v", err)
	}

	readResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_2", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	if err != nil {
		t.Fatalf("storage.file read failed: %v", err)
	}
	if got := readResult["content_text"]; got != "hello file" {
		t.Fatalf("unexpected text content: %#v", got)
	}

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_3", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
		StorageContent:   []byte{0xff, 0x00, 0x01},
	}); err != nil {
		t.Fatalf("storage.file binary write failed: %v", err)
	}

	binaryResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_4", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
	})
	if err != nil {
		t.Fatalf("storage.file binary read failed: %v", err)
	}
	if got := binaryResult["content_base64"]; got != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) {
		t.Fatalf("unexpected base64 content: %#v", got)
	}
}

func TestExecuteStorageFileRejectsMissingScope(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "storage.file",
					ScopeJSON:  `{"storage_roots":[]}`,
				}},
			},
		},
		nil,
		pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_5", runtime.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}
