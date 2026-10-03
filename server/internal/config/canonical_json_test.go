package config

import (
	"math"
	"reflect"
	"testing"
)

func TestCanonicalJSONDocumentPreservesJSONValues(t *testing.T) {
	cfg, err := decodeTypedConfig(defaultDocument())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters = []AdapterInstance{
		{ID: "onebot", Type: AdapterTypeOneBot11, OneBot11: &OneBotConfig{
			ReverseWS: OneBotTransportConfig{AccessToken: "fixture-token"},
		}},
		{ID: "qq", Type: AdapterTypeQQOfficial, QQOfficial: &QQOfficialConfig{Intents: []string{"group_and_c2c"}}},
	}
	invalidStrings := cfg
	invalidStrings.Server.Host = "host\xff\xfe\xed\xa0\x80"
	invalidStrings.Adapters = []AdapterInstance{{ID: "qq\xff", QQOfficial: &QQOfficialConfig{Intents: []string{"intent\xff\xfe"}}}}
	largeNumbers := cfg
	largeNumbers.Runtime.IPCMessageMaxBytes = int(^uint(0) >> 1)
	largeNumbers.Adapter.ReconnectMultiplier = math.SmallestNonzeroFloat64
	nonfinite := cfg
	nonfinite.Adapter.ReconnectMultiplier = math.Inf(1)
	for name, input := range map[string]Config{
		"defaults and adapters": cfg,
		"zero and nil":          {},
		"invalid UTF-8":         invalidStrings,
		"large numbers":         largeNumbers,
		"nonfinite number":      nonfinite,
	} {
		t.Run(name, func(t *testing.T) {
			want := CloneDocument(CanonicalDocumentFromTyped(input))
			got := CanonicalJSONDocumentFromTyped(input)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSON projection differs from serialized values:\ngot %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestCanonicalJSONDocumentOwnsSlicesAndMaps(t *testing.T) {
	cfg := Config{
		Command: &CommandConfig{Prefixes: []string{"/"}},
		Adapters: []AdapterInstance{{ID: "qq", QQOfficial: &QQOfficialConfig{
			AppSecret: "fixture-secret", Intents: []string{"group_and_c2c"},
		}}},
	}
	document := CanonicalJSONDocumentFromTyped(cfg)
	document["command"].(map[string]any)["prefixes"].([]any)[0] = "changed"
	settings := document["adapters"].([]any)[0].(map[string]any)["qqofficial"].(map[string]any)
	settings["intents"].([]any)[0] = "changed"
	settings["app_secret"] = "changed"
	if cfg.Command.Prefixes[0] != "/" || cfg.Adapters[0].QQOfficial.Intents[0] != "group_and_c2c" || cfg.Adapters[0].QQOfficial.AppSecret != "fixture-secret" {
		t.Fatal("returned document changed the typed configuration")
	}
	if !reflect.DeepEqual(CanonicalJSONDocumentFromTyped(cfg), CloneDocument(CanonicalDocumentFromTyped(cfg))) {
		t.Fatal("returned document changed a later projection")
	}
}
