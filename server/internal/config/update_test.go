package config

import (
	"path/filepath"
	"testing"
)

func TestUpdatePrefixNormalization(t *testing.T) {
	for _, raw := range []string{"https://proxy.example/", "https://proxy.example/{url}", "https://proxy.example/https://github.com", "https://proxy.example/https://github.com/RayleaBot/RayleaBot/releases/latest/download/release_manifest.v2.json"} {
		actual, err := NormalizeUpdatePrefix(raw)
		if err != nil || actual != "https://proxy.example" {
			t.Fatalf("%s -> %s, %v", raw, actual, err)
		}
	}
	for _, raw := range []string{"http://proxy.example", "https://user:password@proxy.example", "https://proxy.example/?token=anything", "https://proxy.example/#fragment"} {
		if _, err := NormalizeUpdatePrefix(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestUpdateSettingsRoundTripAndDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	cfg, _, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Update.Mode != "auto" || cfg.Update.Channel != "stable" || len(cfg.Update.Proxies) != 3 {
		t.Fatalf("defaults=%+v", cfg.Update)
	}
	doc := CanonicalDocumentFromTyped(cfg)
	doc["update"].(map[string]any)["proxies"] = []any{"https://custom.example/{url}", "https://custom.example/"}
	saved, _, err := SaveDocument(path, "", doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Update.Proxies) != 1 || saved.Update.Proxies[0] != "https://custom.example" {
		t.Fatalf("saved=%+v", saved.Update)
	}
	doc = CanonicalDocumentFromTyped(saved)
	doc["update"].(map[string]any)["proxies"] = []any{}
	saved, _, err = SaveDocument(path, "", doc)
	if err != nil || len(saved.Update.Proxies) != 0 {
		t.Fatalf("empty proxies not preserved: %+v %v", saved.Update, err)
	}
}
