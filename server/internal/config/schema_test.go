package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedRuntimeSchemasMatchFormalContracts(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	tests := []struct {
		name         string
		contractPath string
		embedded     []byte
	}{
		{
			name:         "backup manifest",
			contractPath: "../../../contracts/backup-manifest.schema.json",
			embedded:     BackupManifestSchemaJSON,
		},
		{
			name:         "config user",
			contractPath: filepath.Join(repoRoot, "contracts", "config.user.schema.json"),
			embedded:     ConfigUserSchemaJSON,
		},
		{
			name:         "plugin info",
			contractPath: filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"),
			embedded:     PluginInfoSchemaJSON,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(tt.contractPath)
			if err != nil {
				t.Fatalf("read contract %s: %v", tt.contractPath, err)
			}
			if !bytes.Equal(normalizeSchemaBytes(content), normalizeSchemaBytes(tt.embedded)) {
				t.Fatalf("embedded schema does not match %s; run node scripts/generate-runtime-schemas.mjs", tt.contractPath)
			}
		})
	}
}

func normalizeSchemaBytes(content []byte) []byte {
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))
}

func TestFormalSchemaFixturesKeepRelativePathConstraints(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	tests := []struct {
		name        string
		schemaPath  string
		fixturePath string
		expectValid bool
	}{
		{
			name:       "plugin info valid fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"plugin-info",
				"ok.minimal-native.json",
			),
			expectValid: true,
		},
		{
			name:       "plugin info legacy contract fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"plugin-info",
				"invalid.legacy-v3.json",
			),
			expectValid: false,
		},
		{
			name:       "plugin management UI path fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"plugin-info",
				"ok.management-ui-and-webhook.json",
			),
			expectValid: true,
		},
		{
			name:       "deps manifest valid fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "deps-manifest.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"deps-manifest",
				"ok.minimal.json",
			),
			expectValid: true,
		},
		{
			name:       "deps manifest parent path fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "deps-manifest.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"deps-manifest",
				"invalid.entrypoint-parent-path.json",
			),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			validator, err := Compile(tt.schemaPath)
			if err != nil {
				t.Fatalf("Compile(%q) error = %v", tt.schemaPath, err)
			}

			fixture, err := LoadJSONFile(tt.fixturePath)
			if err != nil {
				t.Fatalf("LoadJSONFile(%q) error = %v", tt.fixturePath, err)
			}
			document, ok := fixture.(map[string]any)
			if !ok {
				t.Fatalf("fixture %q must decode as an object", tt.fixturePath)
			}

			payload := any(document)
			if input, hasInput := document["input"]; hasInput {
				payload = input
			}

			err = validator.Validate(payload)
			if tt.expectValid && err != nil {
				t.Fatalf("Validate(%q) error = %v", tt.fixturePath, err)
			}
			if !tt.expectValid && err == nil {
				t.Fatalf("Validate(%q) unexpectedly succeeded", tt.fixturePath)
			}
		})
	}
}
