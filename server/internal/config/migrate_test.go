package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMigrateDocumentFoldsBothChatBlocksIntoAdapters(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"schema_version": "3",
		"server":         map[string]any{"host": "127.0.0.1", "port": 8080},
		"onebot": map[string]any{
			"reverse_ws": map[string]any{"enabled": true, "url": "wss://bot.example.com/reverse", "access_token": ""},
			"forward_ws": map[string]any{"enabled": false, "url": "", "access_token": ""},
			"http_api":   map[string]any{"enabled": false, "url": "", "access_token": ""},
			"webhook":    map[string]any{"enabled": false, "url": "", "access_token": ""},
		},
		"qq_official": map[string]any{
			"enabled": true,
			"app_id":  "100000001",
			"intents": []any{"group_and_c2c"},
			"sandbox": false,
		},
	}

	migrated, changed, err := MigrateDocument(document)
	if err != nil {
		t.Fatalf("MigrateDocument() error = %v", err)
	}
	if !changed {
		t.Fatal("MigrateDocument() reported no change for a schema-3 document")
	}
	if got := migrated["schema_version"]; got != "4" {
		t.Fatalf("schema_version = %v, want 4", got)
	}
	if _, present := migrated["onebot"]; present {
		t.Fatal("migrated document kept the onebot section")
	}
	if _, present := migrated["qq_official"]; present {
		t.Fatal("migrated document kept the qq_official section")
	}

	adapters, ok := migrated["adapters"].([]any)
	if !ok || len(adapters) != 2 {
		t.Fatalf("adapters = %#v, want two instances", migrated["adapters"])
	}
	onebot := adapters[0].(map[string]any)
	if onebot["id"] != DefaultOneBot11AdapterID || onebot["type"] != AdapterTypeOneBot11 {
		t.Fatalf("first adapter = %#v, want the default OneBot instance", onebot)
	}
	// The OneBot block had no switch of its own, so the instance is enabled
	// exactly when the install was actually using a transport.
	if onebot["enabled"] != true {
		t.Fatalf("OneBot instance enabled = %v, want true when a transport was on", onebot["enabled"])
	}
	settings := onebot["onebot11"].(map[string]any)
	if got := settings["reverse_ws"].(map[string]any)["url"]; got != "wss://bot.example.com/reverse" {
		t.Fatalf("reverse_ws.url = %v, want the configured URL", got)
	}

	qq := adapters[1].(map[string]any)
	if qq["id"] != DefaultQQOfficialAdapterID || qq["enabled"] != true {
		t.Fatalf("second adapter = %#v, want the enabled QQ instance", qq)
	}
	qqSettings := qq["qqofficial"].(map[string]any)
	// enabled moved up to the instance; leaving a copy behind would fail the
	// settings block, which no longer allows it.
	if _, present := qqSettings["enabled"]; present {
		t.Fatalf("QQ settings kept enabled: %#v", qqSettings)
	}
	if got := qqSettings["app_id"]; got != "100000001" {
		t.Fatalf("app_id = %v, want the configured value", got)
	}
}

func TestMigrateDocumentDisablesOneBotWhenNoTransportWasOn(t *testing.T) {
	t.Parallel()

	migrated, _, err := MigrateDocument(map[string]any{
		"schema_version": "3",
		"onebot": map[string]any{
			"reverse_ws": map[string]any{"enabled": false, "url": ""},
			"forward_ws": map[string]any{"enabled": false, "url": ""},
		},
	})
	if err != nil {
		t.Fatalf("MigrateDocument() error = %v", err)
	}
	adapters := migrated["adapters"].([]any)
	if got := adapters[0].(map[string]any)["enabled"]; got != false {
		t.Fatalf("OneBot instance enabled = %v, want false when every transport was off", got)
	}
}

func TestMigrateDocumentRepointsSecretReferences(t *testing.T) {
	t.Parallel()

	migrated, _, err := MigrateDocument(map[string]any{
		"schema_version": "3",
		"onebot": map[string]any{
			"forward_ws": map[string]any{
				"enabled":      true,
				"access_token": SecretReferenceFor([]string{"onebot", "forward_ws", "access_token"}),
			},
			"reverse_ws": map[string]any{"enabled": false, "access_token": "plaintext-token"},
		},
		"qq_official": map[string]any{
			"enabled":    true,
			"app_secret": SecretReferenceFor([]string{"qq_official", "app_secret"}),
		},
	})
	if err != nil {
		t.Fatalf("MigrateDocument() error = %v", err)
	}
	adapters := migrated["adapters"].([]any)
	settings := adapters[0].(map[string]any)["onebot11"].(map[string]any)

	wantForward := SecretReferenceFor([]string{"adapters", DefaultOneBot11AdapterID, "onebot11", "forward_ws", "access_token"})
	if got := settings["forward_ws"].(map[string]any)["access_token"]; got != wantForward {
		t.Fatalf("forward_ws.access_token = %v, want %q", got, wantForward)
	}
	// A value that is not a reference is a token the operator typed; migration
	// must not turn it into a reference to a secret that was never stored.
	if got := settings["reverse_ws"].(map[string]any)["access_token"]; got != "plaintext-token" {
		t.Fatalf("reverse_ws.access_token = %v, want the plaintext value untouched", got)
	}

	wantSecret := SecretReferenceFor([]string{"adapters", DefaultQQOfficialAdapterID, "qqofficial", "app_secret"})
	if got := adapters[1].(map[string]any)["qqofficial"].(map[string]any)["app_secret"]; got != wantSecret {
		t.Fatalf("app_secret = %v, want %q", got, wantSecret)
	}
}

func TestMigrateDocumentLeavesUnrelatedSectionsAndInputAlone(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"schema_version": "3",
		"log":            map[string]any{"level": "debug", "retention_days": 30},
		"onebot":         map[string]any{"forward_ws": map[string]any{"enabled": true}},
	}
	// The document is cloned through JSON, so the comparison uses the same
	// encoding rather than the Go types the literal above happens to have.
	before := encodeDocument(t, document)

	migrated, _, err := MigrateDocument(document)
	if err != nil {
		t.Fatalf("MigrateDocument() error = %v", err)
	}
	if !reflect.DeepEqual(migrated["log"], CloneDocument(document)["log"]) {
		t.Fatalf("log section = %#v, want it carried through unchanged", migrated["log"])
	}
	if after := encodeDocument(t, document); after != before {
		t.Fatalf("MigrateDocument mutated its input: %s, was %s", after, before)
	}
}

func encodeDocument(t *testing.T, document map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	return string(encoded)
}

func TestMigrateDocumentIsANoOpAtTheCurrentVersion(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"schema_version": "4",
		"adapters":       []any{map[string]any{"id": "onebot11", "type": "onebot11", "enabled": false}},
	}
	migrated, changed, err := MigrateDocument(document)
	if err != nil {
		t.Fatalf("MigrateDocument() error = %v", err)
	}
	if changed {
		t.Fatal("MigrateDocument() reported a change for a current document")
	}
	if !reflect.DeepEqual(migrated, document) {
		t.Fatalf("migrated = %#v, want the document unchanged", migrated)
	}
}

func TestMigrateDocumentRejectsAVersionThisBuildDoesNotKnow(t *testing.T) {
	t.Parallel()

	// A config written by a newer build cannot be understood, and guessing at it
	// would silently discard whatever that build added.
	if _, _, err := MigrateDocument(map[string]any{"schema_version": "5"}); err == nil {
		t.Fatal("MigrateDocument() accepted a future schema version")
	}
	if _, _, err := MigrateDocument(map[string]any{"schema_version": "not-a-version"}); err == nil {
		t.Fatal("MigrateDocument() accepted an unparseable schema version")
	}
}

func TestLoadBacksUpAndRewritesAMigratedConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	if _, _, err := Init(configPath, schemaPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	legacy := map[string]any{
		"schema_version": "3",
		"server":         map[string]any{"host": "127.0.0.1", "port": 8080},
		"onebot": map[string]any{
			"reverse_ws": map[string]any{"enabled": true, "url": "wss://bot.example.com/reverse"},
		},
	}
	writeLegacyConfig(t, configPath, legacy)

	cfg, _, err := Load(configPath, schemaPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	instance, settings, ok := cfg.PrimaryOneBot11()
	if !ok || instance.ID != DefaultOneBot11AdapterID {
		t.Fatalf("PrimaryOneBot11() = %+v, %t, want the migrated instance", instance, ok)
	}
	if settings.ReverseWS.URL != "wss://bot.example.com/reverse" {
		t.Fatalf("ReverseWS.URL = %q, want the configured URL", settings.ReverseWS.URL)
	}

	// The rewritten file is the migrated shape, and the pre-migration file is
	// kept beside it so a downgrade has something to go back to.
	persisted := readYAMLFile(t, configPath)
	if got := persisted["schema_version"]; got != "4" {
		t.Fatalf("persisted schema_version = %v, want 4", got)
	}
	backups, err := filepath.Glob(configPath + ".*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) == 0 {
		t.Fatal("migration did not keep a copy of the pre-migration config")
	}
	backup := readYAMLFile(t, backups[0])
	if !reflect.DeepEqual(backup, legacy) {
		t.Fatalf("backup = %#v, want the pre-migration document", backup)
	}
}

func TestValidateDoesNotPersistAMigration(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	schemaPath := filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")
	if _, _, err := Init(configPath, schemaPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	writeLegacyConfig(t, configPath, map[string]any{
		"schema_version": "3",
		"server":         map[string]any{"host": "127.0.0.1", "port": 8080},
		"onebot":         map[string]any{"reverse_ws": map[string]any{"enabled": false, "url": ""}},
	})
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config before validate: %v", err)
	}

	if _, _, err := Validate(configPath, schemaPath); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after validate: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("Validate() rewrote the config:\n%s", after)
	}
}

func TestLegacyConfigSecretPathCoversOnlyTheMigratedInstances(t *testing.T) {
	t.Parallel()

	forward := []string{"adapters", DefaultOneBot11AdapterID, "onebot11", "forward_ws", "access_token"}
	legacy, ok := LegacyConfigSecretPath(forward)
	if !ok || !reflect.DeepEqual(legacy, []string{"onebot", "forward_ws", "access_token"}) {
		t.Fatalf("LegacyConfigSecretPath(%v) = %v, %t", forward, legacy, ok)
	}
	appSecret := []string{"adapters", DefaultQQOfficialAdapterID, "qqofficial", "app_secret"}
	legacy, ok = LegacyConfigSecretPath(appSecret)
	if !ok || !reflect.DeepEqual(legacy, []string{"qq_official", "app_secret"}) {
		t.Fatalf("LegacyConfigSecretPath(%v) = %v, %t", appSecret, legacy, ok)
	}

	// An instance the operator added has no pre-migration location, so nothing
	// may claim its secret came from one.
	added := []string{"adapters", "second-bot", "onebot11", "forward_ws", "access_token"}
	if legacy, ok := LegacyConfigSecretPath(added); ok {
		t.Fatalf("LegacyConfigSecretPath(%v) = %v, want no legacy path", added, legacy)
	}
}

func writeLegacyConfig(t *testing.T, path string, document map[string]any) {
	t.Helper()
	encoded, err := yaml.Marshal(document)
	if err != nil {
		t.Fatalf("marshal legacy config: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
}

func readYAMLFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return document
}
