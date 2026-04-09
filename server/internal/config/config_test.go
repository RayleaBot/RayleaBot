package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

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
	if cfg.OneBot.WSURL != "" {
		t.Fatalf("OneBot.WSURL = %q, want empty by default", cfg.OneBot.WSURL)
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
	if got := nestedString(t, document, "onebot", "reverse_ws", "url"); got != "" {
		t.Fatalf("onebot.reverse_ws.url = %q, want empty", got)
	}
}

func TestLoadMigratesLegacyConfigShape(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join(t.TempDir(), "config")
	configPath := filepath.Join(configDir, "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	writeYAMLDocument(t, filepath.Join(configDir, "default.yaml"), newPlanningConfigDocument())
	writeYAMLDocument(t, configPath, legacyConfigDocument())

	cfg, _, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Logging.Level != "warn" {
		t.Fatalf("Logging.Level = %q, want warn", cfg.Logging.Level)
	}
	if cfg.Auth.SuperAdmins[0] != "10001" {
		t.Fatalf("Auth.SuperAdmins = %#v, want [10001]", cfg.Auth.SuperAdmins)
	}
	if cfg.Runtime.SchedulerTimezone != "Asia/Shanghai" {
		t.Fatalf("Runtime.SchedulerTimezone = %q, want Asia/Shanghai", cfg.Runtime.SchedulerTimezone)
	}
	if cfg.OneBot.ConnectTimeoutSeconds != 20 {
		t.Fatalf("OneBot.ConnectTimeoutSeconds = %d, want 20", cfg.OneBot.ConnectTimeoutSeconds)
	}

	document, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}

	if _, ok := document["logging"]; ok {
		t.Fatal("legacy logging section should not remain in canonical document")
	}
	if _, ok := document["auth"]; ok {
		t.Fatal("legacy auth section should not remain in canonical document")
	}
	if _, ok := document["cooldown"]; ok {
		t.Fatal("legacy cooldown section should not remain in canonical document")
	}
	if _, ok := document["retention"]; ok {
		t.Fatal("legacy retention section should not remain in canonical document")
	}
	if got := nestedString(t, document, "schema_version"); got != "2" {
		t.Fatalf("schema_version = %q, want 2", got)
	}
	if got := nestedString(t, document, "log", "level"); got != "warn" {
		t.Fatalf("log.level = %q, want warn", got)
	}
	if got := nestedString(t, document, "scheduler", "timezone"); got != "Asia/Shanghai" {
		t.Fatalf("scheduler.timezone = %q, want Asia/Shanghai", got)
	}
	if got := nestedString(t, document, "adapter", "connect_timeout_seconds"); got != "20" {
		t.Fatalf("adapter.connect_timeout_seconds = %q, want 20", got)
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
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
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

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.Server.Port != 8081 {
		t.Fatalf("Server.Port = %d, want 8081", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Auth.DefaultLevel != "group_admin" {
		t.Fatalf("Auth.DefaultLevel = %q, want group_admin", cfg.Auth.DefaultLevel)
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var saved map[string]any
	if err := yaml.Unmarshal(bytes, &saved); err != nil {
		t.Fatalf("parse saved yaml: %v", err)
	}
	if _, ok := saved["logging"]; ok {
		t.Fatal("saved config should not use legacy logging section")
	}
	if _, ok := saved["auth"]; ok {
		t.Fatal("saved config should not use legacy auth section")
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
	document["onebot"].(map[string]any)["ws_url"] = ""
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["url"] = ""
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["enabled"] = false
	delete(document["onebot"].(map[string]any), "access_token")

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.OneBot.WSURL != "" {
		t.Fatalf("OneBot.WSURL = %q, want empty", cfg.OneBot.WSURL)
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if got := nestedString(t, saved, "onebot", "reverse_ws", "url"); got != "" {
		t.Fatalf("saved onebot.reverse_ws.url = %q, want empty", got)
	}
}

func TestSaveDocumentNormalizesShorthandOneBotConnection(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	document["onebot"].(map[string]any)["ws_url"] = "ws:127.0.0.1:2658"
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["url"] = "ws:127.0.0.1:2658"
	document["onebot"].(map[string]any)["reverse_ws"].(map[string]any)["enabled"] = true

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if cfg.OneBot.WSURL != "ws://127.0.0.1:2658" {
		t.Fatalf("OneBot.WSURL = %q, want ws://127.0.0.1:2658", cfg.OneBot.WSURL)
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if got := nestedString(t, saved, "onebot", "reverse_ws", "url"); got != "ws://127.0.0.1:2658" {
		t.Fatalf("saved onebot.reverse_ws.url = %q, want ws://127.0.0.1:2658", got)
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
			"provider":     "standard",
			"ws_url":       "",
			"access_token": "",
			"reverse_ws": map[string]any{
				"enabled": false,
				"url":     "",
			},
			"forward_ws": map[string]any{
				"enabled": false,
				"url":     "",
			},
			"http_api": map[string]any{
				"enabled": false,
				"url":     "",
			},
			"webhook": map[string]any{
				"enabled": false,
				"url":     "",
			},
			"sse": map[string]any{
				"enabled": false,
				"url":     "",
			},
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
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
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
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

func legacyConfigDocument() map[string]any {
	return map[string]any{
		"schema_version": "1",
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"onebot": map[string]any{
			"ws_url":                    "ws://127.0.0.1:6700",
			"access_token":              "",
			"connect_timeout_seconds":   20,
			"reconnect_initial_seconds": 3,
			"reconnect_multiplier":      2,
			"reconnect_max_seconds":     120,
			"reconnect_jitter_ratio":    0.2,
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"storage": map[string]any{
			"kv_value_max_bytes":           65536,
			"kv_total_limit_mb":            16,
			"file_max_bytes":               10485760,
			"plugin_workdir_soft_limit_mb": 256,
		},
		"http": map[string]any{
			"timeout_seconds":     10,
			"max_retries":         2,
			"allow_private_hosts": []string{},
		},
		"logging": map[string]any{
			"level":                 "warn",
			"retention_days":        7,
			"rate_limit_per_plugin": "200/10s",
		},
		"auth": map[string]any{
			"super_admins":              []string{"10001"},
			"default_level":             "group_admin",
			"auto_grant_capabilities":   []string{"logger.write"},
			"session_ttl_days":          14,
			"sliding_renewal":           false,
			"max_sessions":              2,
			"login_fail_limit":          4,
			"login_fail_window_seconds": 120,
		},
		"runtime": map[string]any{
			"scheduler_timezone":                    "Asia/Shanghai",
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
		"render": map[string]any{
			"worker_count":               1,
			"browser_args":               []string{"--disable-gpu"},
			"browser_path":               "",
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
		},
		"web": map[string]any{
			"exposure_mode":    "localhost_only",
			"setup_local_only": true,
		},
		"backup": map[string]any{
			"default_consistency": "offline",
		},
		"retention": map[string]any{
			"audit_logs_retention_days":     90,
			"event_records_retention_days":  7,
			"download_cache_retention_days": 15,
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
		},
		"cooldown": map[string]any{
			"user_command_rate_limit":  "11/60s",
			"group_command_rate_limit": "31/60s",
			"cooldown_reply":           true,
		},
	}
}
