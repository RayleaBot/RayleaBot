package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultsPreserveExplicitZeroFalseAndEmptyCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	writeYAMLDocument(t, path, map[string]any{
		"admin":                map[string]any{"sliding_renewal": false},
		"third_party_accounts": map[string]any{"credential_check_interval_minutes": 0},
		"command":              map[string]any{"prefixes": []any{}},
		"builtin_features":     map[string]any{"menu": map[string]any{"commands": []any{}}},
		"render":               map[string]any{"browser_args": []any{}, "footer_template": ""},
	})
	cfg, _, err := Normalize(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Admin.SlidingRenewal || cfg.ThirdParty.CredentialCheckIntervalMinutes != 0 ||
		len(cfg.Command.Prefixes) != 0 || len(cfg.Builtin.Menu.Commands) != 0 ||
		len(cfg.Render.BrowserArgs) != 0 || cfg.Render.FooterTemplate != "" {
		t.Fatal("normalization replaced explicitly configured empty or disabled values")
	}
	document := CanonicalDocumentFromTyped(cfg)
	if _, _, err := SaveDocument(path, "", document); err != nil {
		t.Fatal(err)
	}
	roundTrip, _, err := Load(path, "")
	if err != nil || !reflect.DeepEqual(cfg, roundTrip) {
		t.Fatalf("typed/document/file round trip changed the configuration: %v", err)
	}
}

func TestInitializationWritesStableYAMLIntegers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	if _, _, err := Init(path, ""); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := yaml.Unmarshal(before, &persisted); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ipc_message_max_bytes", "stderr_rate_limit_bytes_per_second"} {
		if _, ok := persisted["runtime"].(map[string]any)[key].(int); !ok {
			t.Fatalf("%s was not written as a YAML integer", key)
		}
	}
	if _, _, err := Normalize(path, ""); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("second normalization changed the initialized file")
	}
}

func TestTransportNormalizationDoesNotHideInvalidInput(t *testing.T) {
	for _, value := range []any{"false", nil, 0} {
		document := newPlanningConfigDocument()
		planningOneBot(t, document)["forward_ws"].(map[string]any)["enabled"] = value
		if _, _, _, err := NormalizeDocument("user.yaml", "", document); err == nil {
			t.Fatalf("accepted enabled=%#v", value)
		}
	}
}

func TestRateLimitOverflowIsRejectedBeforePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	document := defaultDocument()
	document["message"].(map[string]any)["rate_limit_per_plugin"] = "999999999999999999999999999999/1s"
	if _, _, err := SaveDocument(path, "", document); err == nil || !strings.Contains(err.Error(), "rate_limit_per_plugin") {
		t.Fatalf("overflowed rate limit result: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid settings were persisted")
	}
}
