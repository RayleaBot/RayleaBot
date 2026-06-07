package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadAndSaveUseEmbeddedSchemaByDefault(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	cfg, summary, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Load with embedded schema failed: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("server.host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if summary.SchemaPath != "builtin://contracts/config.user.schema.json" {
		t.Fatalf("summary.SchemaPath = %q", summary.SchemaPath)
	}

	document, err := LoadDocument(configPath, "")
	if err != nil {
		t.Fatalf("LoadDocument with embedded schema failed: %v", err)
	}
	server, ok := document["server"].(map[string]any)
	if !ok {
		t.Fatalf("server section = %#v", document["server"])
	}
	server["port"] = 18080

	cfg, summary, err = SaveDocument(configPath, "", document)
	if err != nil {
		t.Fatalf("SaveDocument with embedded schema failed: %v", err)
	}
	if cfg.Server.Port != 18080 {
		t.Fatalf("server.port = %d, want 18080", cfg.Server.Port)
	}
	if summary.SchemaPath != "builtin://contracts/config.user.schema.json" {
		t.Fatalf("summary.SchemaPath after save = %q", summary.SchemaPath)
	}
}

func TestLoadBootstrapsDefaultAndUserConfigWhenMissing(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	cfg, _, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.OneBot.ForwardWS.URL != "" {
		t.Fatalf("OneBot.ForwardWS.URL = %q, want empty by default", cfg.OneBot.ForwardWS.URL)
	}

	defaultPath := filepath.Join(filepath.Dir(configPath), "default.yaml")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Fatalf("default.yaml was not created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("user.yaml was not created: %v", err)
	}

	document, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}

	if got := nestedString(t, document, "schema_version"); got != "2" {
		t.Fatalf("schema_version = %q, want 2", got)
	}
	if _, ok := document["log"]; !ok {
		t.Fatal("expected planning-aligned log section in persisted document")
	}
	if _, ok := document["admin"]; !ok {
		t.Fatal("expected planning-aligned admin section in persisted document")
	}
	if _, ok := document["adapter"]; !ok {
		t.Fatal("expected planning-aligned adapter section in persisted document")
	}
	if got := nestedString(t, document, "builtin_features", "menu", "commands"); got != `["help","帮助"]` {
		t.Fatalf("builtin_features.menu.commands = %q, want default help commands", got)
	}
	if got := nestedString(t, document, "builtin_features", "menu", "prefixes"); got != "[]" {
		t.Fatalf("builtin_features.menu.prefixes = %q, want []", got)
	}
	if got := nestedString(t, document, "onebot", "reverse_ws", "url"); got != "" {
		t.Fatalf("onebot.reverse_ws.url = %q, want empty", got)
	}
	if got := nestedString(t, document, "onebot", "forward_ws", "access_token"); got != "" {
		t.Fatalf("onebot.forward_ws.access_token = %q, want empty", got)
	}
	if got := nestedString(t, document, "render", "footer_template"); got != DefaultRenderFooterTemplate {
		t.Fatalf("render.footer_template = %q, want default footer template", got)
	}
	if got := nestedString(t, document, "render", "default_output"); got != DefaultRenderOutput {
		t.Fatalf("render.default_output = %q, want %s", got, DefaultRenderOutput)
	}
	if got := nestedString(t, document, "render", "device_scale_percent"); got != "100" {
		t.Fatalf("render.device_scale_percent = %q, want 100", got)
	}
}

func TestLoadMergesDefaultAndUserOverrides(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join(t.TempDir(), "config")
	configPath := filepath.Join(configDir, "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	defaultDoc := newPlanningConfigDocument()
	defaultDoc["server"].(map[string]any)["host"] = "127.0.0.1"
	defaultDoc["server"].(map[string]any)["port"] = 8080
	defaultDoc["log"].(map[string]any)["level"] = "info"
	writeYAMLDocument(t, filepath.Join(configDir, "default.yaml"), defaultDoc)

	override := map[string]any{
		"schema_version": "2",
		"server": map[string]any{
			"port": 9090,
		},
		"log": map[string]any{
			"level": "debug",
		},
	}
	writeYAMLDocument(t, configPath, override)

	cfg, _, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want debug", cfg.Log.Level)
	}

	document, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if got := nestedString(t, document, "server", "host"); got != "127.0.0.1" {
		t.Fatalf("server.host = %q, want 127.0.0.1", got)
	}
	if got := nestedString(t, document, "server", "port"); got != "9090" {
		t.Fatalf("server.port = %q, want 9090", got)
	}
	if got := nestedString(t, document, "log", "level"); got != "debug" {
		t.Fatalf("log.level = %q, want debug", got)
	}
}

func TestSaveDocumentPersistsPlanningAlignedShape(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	document["server"].(map[string]any)["port"] = 8081
	document["log"].(map[string]any)["level"] = "debug"
	document["permission"].(map[string]any)["default_level"] = "group_admin"
	document["user"].(map[string]any)["cooldown_reply"] = false
	document["builtin_features"].(map[string]any)["menu"].(map[string]any)["commands"] = []string{"menu", "菜单"}
	document["builtin_features"].(map[string]any)["menu"].(map[string]any)["prefixes"] = []string{"#", "！"}

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.Server.Port != 8081 {
		t.Fatalf("Server.Port = %d, want 8081", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want debug", cfg.Log.Level)
	}
	if cfg.Permission.DefaultLevel != "group_admin" {
		t.Fatalf("Permission.DefaultLevel = %q, want group_admin", cfg.Permission.DefaultLevel)
	}
	if !reflect.DeepEqual(cfg.Builtin.Menu.Commands, []string{"menu", "菜单"}) {
		t.Fatalf("Builtin.Menu.Commands = %#v, want [menu 菜单]", cfg.Builtin.Menu.Commands)
	}
	if !reflect.DeepEqual(cfg.Builtin.Menu.Prefixes, []string{"#", "！"}) {
		t.Fatalf("Builtin.Menu.Prefixes = %#v, want [# ！]", cfg.Builtin.Menu.Prefixes)
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var saved map[string]any
	if err := yaml.Unmarshal(bytes, &saved); err != nil {
		t.Fatalf("parse saved yaml: %v", err)
	}
	if got := nestedString(t, saved, "schema_version"); got != "2" {
		t.Fatalf("schema_version = %q, want 2", got)
	}
}

func TestSaveDocumentAllowsBlankOneBotConnection(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["url"] = ""
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["enabled"] = false
	delete(document["onebot"].(map[string]any)["forward_ws"].(map[string]any), "access_token")

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.OneBot.ForwardWS.URL != "" {
		t.Fatalf("OneBot.ForwardWS.URL = %q, want empty", cfg.OneBot.ForwardWS.URL)
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if got := nestedString(t, saved, "onebot", "forward_ws", "url"); got != "" {
		t.Fatalf("saved onebot.forward_ws.url = %q, want empty", got)
	}
	if got := nestedString(t, saved, "onebot", "forward_ws", "access_token"); got != "" {
		t.Fatalf("saved onebot.forward_ws.access_token = %q, want empty", got)
	}
}

func TestSaveDocumentPreservesDisabledConfiguredOneBotTransports(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["enabled"] = false
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["url"] = "ws://127.0.0.1:2658"
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["enabled"] = false
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["url"] = "wss://example.com/reverse"

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.OneBot.ForwardWS.Enabled {
		t.Fatal("OneBot.ForwardWS.Enabled = true, want false")
	}
	if cfg.OneBot.ReverseWS.Enabled {
		t.Fatal("OneBot.ReverseWS.Enabled = true, want false")
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if got := nestedString(t, saved, "onebot", "forward_ws", "enabled"); got != "false" {
		t.Fatalf("saved onebot.forward_ws.enabled = %q, want false", got)
	}
	if got := nestedString(t, saved, "onebot", "reverse_ws", "enabled"); got != "false" {
		t.Fatalf("saved onebot.reverse_ws.enabled = %q, want false", got)
	}
}

func TestSaveDocumentRejectsInvalidRenderDeviceScalePercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
	}{
		{name: "below_minimum", value: 49},
		{name: "above_maximum", value: 501},
		{name: "non_integer", value: 100.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
			schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
			document := newPlanningConfigDocument()
			document["render"].(map[string]any)["device_scale_percent"] = tt.value

			if _, _, err := SaveDocument(configPath, schemaPath, document); err == nil {
				t.Fatalf("SaveDocument accepted render.device_scale_percent=%v", tt.value)
			}
		})
	}
}

func TestSaveDocumentAcceptsRenderOutputAndDeviceScalePercent(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	renderDoc := document["render"].(map[string]any)
	renderDoc["default_output"] = "jpeg"
	renderDoc["device_scale_percent"] = 500

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.Render.DefaultOutput != "jpeg" {
		t.Fatalf("Render.DefaultOutput = %q, want jpeg", cfg.Render.DefaultOutput)
	}
	if cfg.Render.DeviceScalePercent != 500 {
		t.Fatalf("Render.DeviceScalePercent = %d, want 500", cfg.Render.DeviceScalePercent)
	}
}

func TestLoadHealsNullPlanningAlignedValues(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join(t.TempDir(), "config")
	configPath := filepath.Join(configDir, "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	defaultDoc := newPlanningConfigDocument()
	defaultDoc["adapter"].(map[string]any)["connect_timeout_seconds"] = nil
	defaultDoc["adapter"].(map[string]any)["reconnect_initial_seconds"] = nil
	defaultDoc["adapter"].(map[string]any)["reconnect_multiplier"] = nil
	defaultDoc["adapter"].(map[string]any)["reconnect_max_seconds"] = nil
	defaultDoc["adapter"].(map[string]any)["reconnect_jitter_ratio"] = nil
	defaultDoc["scheduler"].(map[string]any)["timezone"] = nil
	writeYAMLDocument(t, filepath.Join(configDir, "default.yaml"), defaultDoc)
	writeYAMLDocument(t, configPath, map[string]any{
		"schema_version": "2",
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
	})

	cfg, _, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Adapter.ConnectTimeoutSeconds != 15 {
		t.Fatalf("Adapter.ConnectTimeoutSeconds = %d, want 15", cfg.Adapter.ConnectTimeoutSeconds)
	}
	if cfg.Adapter.ReconnectInitialSeconds != 2 {
		t.Fatalf("Adapter.ReconnectInitialSeconds = %d, want 2", cfg.Adapter.ReconnectInitialSeconds)
	}
	if cfg.Adapter.ReconnectMultiplier != 2 {
		t.Fatalf("Adapter.ReconnectMultiplier = %v, want 2", cfg.Adapter.ReconnectMultiplier)
	}
	if cfg.Adapter.ReconnectMaxSeconds != 120 {
		t.Fatalf("Adapter.ReconnectMaxSeconds = %d, want 120", cfg.Adapter.ReconnectMaxSeconds)
	}
	if cfg.Adapter.ReconnectJitterRatio != 0.2 {
		t.Fatalf("Adapter.ReconnectJitterRatio = %v, want 0.2", cfg.Adapter.ReconnectJitterRatio)
	}
	if cfg.Scheduler.Timezone != "" {
		t.Fatalf("Scheduler.Timezone = %q, want empty", cfg.Scheduler.Timezone)
	}
}

func nestedString(t *testing.T, document map[string]any, path ...string) string {
	t.Helper()

	var current any = document
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v is not an object at %q: %#v", path, segment, current)
		}
		current = object[segment]
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatalf("marshal nested value %v: %v", path, err)
	}
	var text string
	if err := json.Unmarshal(encoded, &text); err == nil {
		return text
	}
	return string(bytesTrim(encoded))
}

func bytesTrim(raw []byte) []byte {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func writeYAMLDocument(t *testing.T, path string, document map[string]any) {
	t.Helper()

	bytes, err := yaml.Marshal(document)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newPlanningConfigDocument() map[string]any {
	return map[string]any{
		"schema_version": "2",
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"onebot": map[string]any{
			"reverse_ws": map[string]any{
				"enabled":      false,
				"url":          "",
				"access_token": "",
			},
			"forward_ws": map[string]any{
				"enabled":      false,
				"url":          "",
				"access_token": "",
			},
			"http_api": map[string]any{
				"enabled":      false,
				"url":          "",
				"access_token": "",
			},
			"webhook": map[string]any{
				"enabled":      false,
				"url":          "",
				"access_token": "",
			},
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
		},
		"builtin_features": map[string]any{
			"menu": map[string]any{
				"commands": []string{"help", "帮助"},
				"prefixes": []string{},
			},
		},
		"admin": map[string]any{
			"super_admins":              []string{},
			"session_ttl_days":          7,
			"sliding_renewal":           true,
			"max_sessions":              3,
			"login_fail_limit":          5,
			"login_fail_window_seconds": 300,
		},
		"permission": map[string]any{
			"default_level":           "everyone",
			"auto_grant_capabilities": []string{},
		},
		"render": map[string]any{
			"worker_count":               1,
			"browser_args":               []string{"--disable-gpu"},
			"browser_path":               "",
			"default_output":             DefaultRenderOutput,
			"device_scale_percent":       DefaultRenderDeviceScalePercent,
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
			"footer_template":            DefaultRenderFooterTemplate,
		},
		"scheduler": map[string]any{
			"timezone": "",
		},
		"runtime": map[string]any{
			"plugin_init_timeout_seconds":           30,
			"plugin_init_max_total_seconds":         300,
			"plugin_event_timeout_seconds":          60,
			"max_pending_events_per_plugin":         16,
			"max_pending_control_events_per_plugin": 4,
			"nodejs_max_old_space_size_mb":          256,
			"dependency_install_timeout_seconds":    900,
			"max_concurrent_dependency_installs":    1,
			"ipc_pending_actions_max":               256,
			"ipc_action_burst_limit":                "100/1s",
			"stderr_rate_limit_bytes_per_second":    262144,
			"max_concurrent_tasks_per_plugin":       4,
			"crash_backoff_initial_seconds":         2,
			"crash_backoff_max_seconds":             60,
			"shutdown_grace_seconds":                10,
			"ipc_message_max_bytes":                 8388608,
		},
		"storage": map[string]any{
			"kv_value_max_bytes":           65536,
			"kv_total_limit_mb":            16,
			"file_max_bytes":               10485760,
			"plugin_workdir_soft_limit_mb": 256,
		},
		"data": map[string]any{
			"audit_logs_retention_days":     90,
			"event_records_retention_days":  7,
			"download_cache_retention_days": 15,
		},
		"log": map[string]any{
			"level":                 "info",
			"retention_days":        7,
			"rate_limit_per_plugin": "200/10s",
		},
		"message": map[string]any{
			"rate_limit_per_plugin":   "20/10s",
			"rate_limit_per_target":   "5/5s",
			"circuit_breaker_seconds": 30,
		},
		"user": map[string]any{
			"command_rate_limit": "10/60s",
			"cooldown_reply":     true,
		},
		"group": map[string]any{
			"command_rate_limit": "30/60s",
		},
		"adapter": map[string]any{
			"reconnect_initial_seconds": 2,
			"reconnect_multiplier":      2,
			"reconnect_max_seconds":     120,
			"reconnect_jitter_ratio":    0.2,
			"connect_timeout_seconds":   15,
		},
		"http": map[string]any{
			"timeout_seconds":     10,
			"max_retries":         2,
			"allow_private_hosts": []string{},
		},
		"web": map[string]any{
			"exposure_mode":    "localhost_only",
			"setup_local_only": true,
		},
		"backup": map[string]any{
			"default_consistency": "offline",
		},
	}
}
