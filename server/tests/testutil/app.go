package testutil

import (
	"encoding/json"
	"path/filepath"
	"testing"
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
