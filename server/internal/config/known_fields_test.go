package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUnknownConfigFieldsAreIgnored(t *testing.T) {
	t.Parallel()
	for _, schemaPath := range []string{"", filepath.Join("..", "..", "..", "contracts", "config.user.schema.json")} {
		t.Run(schemaPath, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "user.yaml")
			input := newPlanningConfigDocument()
			writeYAMLDocument(t, configPath, input)
			want, err := LoadDocument(configPath, schemaPath)
			if err != nil {
				t.Fatal(err)
			}
			addUnknownConfigFields(t, input)
			originalInput := CloneDocument(input)
			writeYAMLDocument(t, configPath, input)
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			got, err := LoadDocument(configPath, schemaPath)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("LoadDocument did not ignore only unknown fields: err=%v", err)
			}
			if _, _, err := Validate(configPath, schemaPath); err != nil {
				t.Fatalf("Validate rejected unknown fields: %v", err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("read-only configuration operations changed the file: %v", err)
			}
			if _, _, err := Normalize(configPath, schemaPath); err != nil {
				t.Fatalf("Normalize rejected unknown fields: %v", err)
			}
			assertPersistedConfigDocument(t, configPath, want)
			if _, _, err := SaveDocument(configPath, schemaPath, input); err != nil {
				t.Fatalf("SaveDocument rejected unknown fields: %v", err)
			}
			assertPersistedConfigDocument(t, configPath, want)
			if !reflect.DeepEqual(CloneDocument(input), originalInput) {
				t.Fatal("SaveDocument mutated the caller's document")
			}
		})
	}
}

func TestUnknownConfigFieldsDoNotBypassKnownFieldValidation(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(map[string]any){
		"type":    func(doc map[string]any) { doc["server"].(map[string]any)["port"] = "invalid" },
		"range":   func(doc map[string]any) { doc["server"].(map[string]any)["port"] = 70000 },
		"enum":    func(doc map[string]any) { doc["render"].(map[string]any)["default_output"] = "unknown" },
		"null":    func(doc map[string]any) { doc["adapter"].(map[string]any)["connect_timeout_seconds"] = nil },
		"runtime": func(doc map[string]any) { doc["scheduler"].(map[string]any)["timezone"] = "Invalid/Timezone" },
	} {
		t.Run(name, func(t *testing.T) {
			input := newPlanningConfigDocument()
			addUnknownConfigFields(t, input)
			mutate(input)
			configPath := filepath.Join(t.TempDir(), "user.yaml")
			writeYAMLDocument(t, configPath, input)
			if _, _, err := Load(configPath, ""); err == nil {
				t.Fatal("Load accepted an invalid declared field")
			}
			if _, _, err := SaveDocument(configPath, "", input); err == nil {
				t.Fatal("SaveDocument accepted an invalid declared field")
			}
		})
	}
}

func TestUnknownConfigSectionCanContainNonJSONYAML(t *testing.T) {
	t.Parallel()
	configPath := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 9090\nobsolete_section:\n  true: ignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(configPath, "")
	if err != nil || cfg.Server.Port != 9090 {
		t.Fatalf("unknown section interfered with declared configuration: port=%d err=%v", cfg.Server.Port, err)
	}
}

func addUnknownConfigFields(t *testing.T, input map[string]any) {
	t.Helper()
	input["obsolete_section"] = map[string]any{"ignored": []any{nil, true, "unused"}}
	input["server"].(map[string]any)["obsolete_port"] = nil
	web := input["web"].(map[string]any)
	web["exposure_mode"] = "localhost_only"
	web["public_origin"] = ""
	web["setup_local_only"] = true
	web["trusted_proxy_cidrs"] = []any{}
	adapter := input["adapters"].([]any)[0].(map[string]any)
	adapter["obsolete_adapter_flag"] = true
	planningOneBot(t, input)["reverse_ws"].(map[string]any)["obsolete_transport_option"] = map[string]any{"ignored": true}
}

func assertPersistedConfigDocument(t *testing.T, configPath string, want map[string]any) {
	t.Helper()
	raw, _, err := readYAMLDocument(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := normalizeDocument(raw)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted document retained unknown fields or changed declared values: %v", err)
	}
}
