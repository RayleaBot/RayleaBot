package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileAndValidateSimpleSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	schemaPath := filepath.Join(root, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": { "type": "string" }
  }
}`), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	validator, err := Compile(schemaPath)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if validator.Path() == "" {
		t.Fatal("expected absolute validator path")
	}
	if err := validator.Validate(map[string]any{"name": "raylea"}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := validator.Validate(map[string]any{}); err == nil {
		t.Fatal("expected validation failure for missing required field")
	}
}

func TestCompileFormalSchemasUsedByRuntime(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, relativePath := range []string{
		filepath.Join("contracts", "plugin-info.schema.json"),
		filepath.Join("contracts", "deps-manifest.schema.json"),
	} {
		relativePath := relativePath
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()

			validator, err := Compile(filepath.Join(repoRoot, relativePath))
			if err != nil {
				t.Fatalf("Compile(%q) error = %v", relativePath, err)
			}
			if validator.Path() == "" {
				t.Fatalf("expected absolute validator path for %q", relativePath)
			}
		})
	}
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
				"ok.minimal-python.json",
			),
			expectValid: true,
		},
		{
			name:       "plugin info parent path fixture",
			schemaPath: filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"),
			fixturePath: filepath.Join(
				repoRoot,
				"fixtures",
				"plugin-info",
				"invalid.icon-parent-path.json",
			),
			expectValid: false,
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
