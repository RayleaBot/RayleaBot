package testutil

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func WritePersistentYAMLConfig(t testing.TB, databasePath string) string {
	t.Helper()

	fixture := LoadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}

	database := input["database"].(map[string]any)
	database["path"] = databasePath

	return WriteYAMLConfigMap(t, input)
}

func ResponseBodyString(t testing.TB, body map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	return string(encoded)
}

func NormalizeJSONMap(t testing.TB, body map[string]any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json map: %v", err)
	}
	var normalized map[string]any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatalf("normalize json map: %v", err)
	}
	return normalized
}

func MustYAML(t testing.TB, value any) []byte {
	t.Helper()

	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	return data
}
