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
	if cfg.ThirdParty.CredentialCheckIntervalMinutes != 360 {
		t.Fatalf("third_party_accounts.credential_check_interval_minutes = %d, want 360", cfg.ThirdParty.CredentialCheckIntervalMinutes)
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

func TestLoadDoesNotWriteConfigFilesWhenMissing(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(filepath.Dir(configPath), "default.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Load should not create default.yaml, stat err = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("Load should not create user.yaml, stat err = %v", err)
	}
}

func TestNormalizeBootstrapsDefaultAndUserConfigWhenMissing(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	cfg, _, err := Normalize(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if len(cfg.Adapters) != 0 {
		t.Fatalf("Adapters = %#v, want none by default", cfg.Adapters)
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

	if got := nestedString(t, document, "schema_version"); got != "4" {
		t.Fatalf("schema_version = %q, want 4", got)
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
	// A fresh install configures no adapter: the bot accepts chat traffic only
	// once the operator adds one.
	if got := nestedString(t, document, "adapters"); got != "[]" {
		t.Fatalf("adapters = %q, want []", got)
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

func TestInitWritesCanonicalConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	if _, _, err := Init(configPath, schemaPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(configPath), "default.yaml")); err != nil {
		t.Fatalf("default.yaml was not created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("user.yaml was not created: %v", err)
	}
}

func TestValidateDoesNotRewriteConfig(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join(t.TempDir(), "config")
	configPath := filepath.Join(configDir, "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	writeYAMLDocument(t, configPath, map[string]any{
		"schema_version": "4",
		"server": map[string]any{
			"port": 9090,
		},
	})
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config before validate: %v", err)
	}

	cfg, _, err := Validate(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after validate: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("Validate rewrote config:\nbefore=%s\nafter=%s", before, after)
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
		"schema_version": "4",
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
	if got := nestedString(t, saved, "schema_version"); got != "4" {
		t.Fatalf("schema_version = %q, want 4", got)
	}
}

func TestSaveDocumentAllowsBlankOneBotConnection(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	forwardWS := planningOneBot(t, document)["forward_ws"].(map[string]any)
	forwardWS["url"] = ""
	forwardWS["enabled"] = false
	delete(forwardWS, "access_token")

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if got := onebotConfig(t, cfg).ForwardWS.URL; got != "" {
		t.Fatalf("OneBot.ForwardWS.URL = %q, want empty", got)
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	savedForwardWS := adapterSettings(t, saved, DefaultOneBot11AdapterID, AdapterTypeOneBot11)["forward_ws"]
	if got := nestedString(t, savedForwardWS.(map[string]any), "url"); got != "" {
		t.Fatalf("saved forward_ws.url = %q, want empty", got)
	}
	if got := nestedString(t, savedForwardWS.(map[string]any), "access_token"); got != "" {
		t.Fatalf("saved forward_ws.access_token = %q, want empty", got)
	}
}

func TestSaveDocumentPreservesDisabledConfiguredOneBotTransports(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	onebot := planningOneBot(t, document)
	onebot["forward_ws"].(map[string]any)["enabled"] = false
	onebot["forward_ws"].(map[string]any)["url"] = "ws://127.0.0.1:2658"
	onebot["reverse_ws"].(map[string]any)["enabled"] = false
	onebot["reverse_ws"].(map[string]any)["url"] = "wss://example.com/reverse"

	cfg, _, err := SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}
	if onebotConfig(t, cfg).ForwardWS.Enabled {
		t.Fatal("OneBot.ForwardWS.Enabled = true, want false")
	}
	if onebotConfig(t, cfg).ReverseWS.Enabled {
		t.Fatal("OneBot.ReverseWS.Enabled = true, want false")
	}

	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	savedOneBot := adapterSettings(t, saved, DefaultOneBot11AdapterID, AdapterTypeOneBot11)
	if got := nestedString(t, savedOneBot["forward_ws"].(map[string]any), "enabled"); got != "false" {
		t.Fatalf("saved forward_ws.enabled = %q, want false", got)
	}
	if got := nestedString(t, savedOneBot["reverse_ws"].(map[string]any), "enabled"); got != "false" {
		t.Fatalf("saved reverse_ws.enabled = %q, want false", got)
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

func TestSaveDocumentValidatesCredentialCheckInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{name: "disabled", value: 0},
		{name: "minimum", value: 15},
		{name: "maximum", value: 10080},
		{name: "below minimum", value: 1, wantErr: true},
		{name: "above maximum", value: 10081, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := newPlanningConfigDocument()
			thirdParty := document["third_party_accounts"].(map[string]any)
			thirdParty["credential_check_interval_minutes"] = test.value
			_, _, err := SaveDocument(
				filepath.Join(t.TempDir(), "config", "user.yaml"),
				filepath.Join("..", "..", "..", "contracts", "config.user.schema.json"),
				document,
			)
			if test.wantErr && err == nil {
				t.Fatalf("SaveDocument(%d) succeeded, want error", test.value)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("SaveDocument(%d) error = %v", test.value, err)
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
		"schema_version": "4",
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
	if cfg.Scheduler.Timezone != DefaultTimezone {
		t.Fatalf("Scheduler.Timezone = %q, want %q", cfg.Scheduler.Timezone, DefaultTimezone)
	}
}

// planningOneBot returns the OneBot settings of the adapter the planning
// document configures, so a test can edit one transport without restating the
// whole adapters list.
func planningOneBot(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	return adapterSettings(t, document, DefaultOneBot11AdapterID, AdapterTypeOneBot11)
}

func adapterSettings(t *testing.T, document map[string]any, id, block string) map[string]any {
	t.Helper()
	adapters, ok := document["adapters"].([]any)
	if !ok {
		t.Fatalf("document has no adapters list: %#v", document["adapters"])
	}
	for _, entry := range adapters {
		instance, ok := entry.(map[string]any)
		if !ok || instance["id"] != id {
			continue
		}
		settings, ok := instance[block].(map[string]any)
		if !ok {
			t.Fatalf("adapter %q has no %s settings: %#v", id, block, instance)
		}
		return settings
	}
	t.Fatalf("document has no adapter %q", id)
	return nil
}

// onebotConfig returns the settings of the one OneBot adapter a test configured.
func onebotConfig(t *testing.T, cfg Config) OneBotConfig {
	t.Helper()
	settings, ok := cfg.OneBot11Settings(DefaultOneBot11AdapterID)
	if !ok {
		t.Fatal("config has no onebot11 adapter")
	}
	return settings
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
		"schema_version": "4",
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"adapters": []any{map[string]any{
			"id":      DefaultOneBot11AdapterID,
			"type":    AdapterTypeOneBot11,
			"enabled": false,
			"onebot11": map[string]any{
				"reverse_ws": map[string]any{
					"enabled":                   false,
					"url":                       "",
					"access_token":              "",
					"access_token_query_compat": false,
				},
				"forward_ws": map[string]any{
					"enabled":                   false,
					"url":                       "",
					"access_token":              "",
					"access_token_query_compat": false,
				},
				"http_api": map[string]any{
					"enabled":      false,
					"url":          "",
					"access_token": "",
				},
				"webhook": map[string]any{
					"enabled":                   false,
					"url":                       "",
					"access_token":              "",
					"access_token_query_compat": false,
				},
			},
		}},
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
			"session_absolute_ttl_days": 30,
			"sliding_renewal":           true,
			"max_sessions":              3,
			"login_fail_limit":          5,
			"login_fail_window_seconds": 300,
		},
		"permission": map[string]any{
			"default_level": "everyone",
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
		"third_party_accounts": map[string]any{
			"credential_check_interval_minutes": 360,
			"douyin_login": map[string]any{
				"browser_mode":         "auto",
				"remote_debugging_url": "",
			},
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
			"timeout_seconds":         10,
			"max_retries":             2,
			"max_response_body_bytes": int64(4194304),
			"allow_private_hosts":     []string{},
		},
		"web": map[string]any{
			"exposure_mode":             "localhost_only",
			"setup_local_only":          true,
			"public_origin":             "",
			"plugin_ui_origin_template": "",
			"trusted_proxy_cidrs":       []string{},
		},
		"backup": map[string]any{
			"default_consistency": "offline",
		},
	}
}

func TestSaveDocumentTreatsQQOfficialAsOptional(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")

	// No QQ instance: a config that configures only OneBot still saves, and the
	// QQ accessor reports nothing rather than a zero-valued adapter.
	cfg, _, err := SaveDocument(configPath, schemaPath, newPlanningConfigDocument())
	if err != nil {
		t.Fatalf("SaveDocument() without a QQ adapter error = %v", err)
	}
	if _, ok := cfg.QQOfficialSettings(DefaultQQOfficialAdapterID); ok {
		t.Fatal("QQOfficialSettings reported settings for an adapter that was never configured")
	}
	saved, err := LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	if adapters, ok := saved["adapters"].([]any); !ok || len(adapters) != 1 {
		t.Fatalf("saved adapters = %#v, want only the configured OneBot adapter", saved["adapters"])
	}

	// Configured instance: values reach the typed config, and the secret stays
	// a reference rather than a plaintext value.
	document := newPlanningConfigDocument()
	document["adapters"] = append(document["adapters"].([]any), map[string]any{
		"id":      DefaultQQOfficialAdapterID,
		"type":    AdapterTypeQQOfficial,
		"enabled": true,
		"qqofficial": map[string]any{
			"app_id":     "100000001",
			"app_secret": "secret://adapters/qq-official/qqofficial/app_secret",
			"intents":    []any{"group_and_c2c"},
			"sandbox":    false,
		},
	})
	cfg, _, err = SaveDocument(configPath, schemaPath, document)
	if err != nil {
		t.Fatalf("SaveDocument() with a QQ adapter error = %v", err)
	}
	instance, ok := cfg.AdapterByID(DefaultQQOfficialAdapterID)
	if !ok || !instance.Enabled {
		t.Fatalf("adapter %q = %+v, want an enabled instance", DefaultQQOfficialAdapterID, instance)
	}
	settings, ok := cfg.QQOfficialSettings(DefaultQQOfficialAdapterID)
	if !ok || settings.AppID != "100000001" {
		t.Fatalf("QQOfficialSettings = %+v, %t, want the configured app id", settings, ok)
	}
	if len(settings.Intents) != 1 || settings.Intents[0] != "group_and_c2c" {
		t.Fatalf("QQOfficial.Intents = %v, want [group_and_c2c]", settings.Intents)
	}
}

func TestSaveDocumentRejectsPartialQQOfficialBlock(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	document := newPlanningConfigDocument()
	// The settings block has required fields of its own; a half-written block
	// must not silently start an adapter with defaults nobody chose.
	document["adapters"] = append(document["adapters"].([]any), map[string]any{
		"id":         DefaultQQOfficialAdapterID,
		"type":       AdapterTypeQQOfficial,
		"enabled":    true,
		"qqofficial": map[string]any{"app_id": "100000001"},
	})

	if _, _, err := SaveDocument(configPath, schemaPath, document); err == nil {
		t.Fatal("SaveDocument() accepted a QQ settings block missing required fields")
	}
}
