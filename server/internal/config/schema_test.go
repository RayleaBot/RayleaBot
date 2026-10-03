package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
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

func TestBuiltinConfigValidatorSupportsConcurrentIndependentDocuments(t *testing.T) {
	for range 32 {
		t.Run("validate", func(t *testing.T) {
			t.Parallel()
			validator, err := CompileBuiltin(ConfigUserSchemaID)
			if err != nil {
				t.Fatal(err)
			}
			document := CloneDocument(defaultDocument())
			if err := validator.Validate(document); err != nil {
				t.Fatalf("valid configuration: %v", err)
			}
			document["server"].(map[string]any)["port"] = "invalid-port"
			if err := validator.Validate(document); err == nil {
				t.Fatal("accepted an invalid port")
			}
		})
	}
}

func TestBuiltinValidationErrorsCannotMutateSharedSchema(t *testing.T) {
	for range 8 {
		t.Run("independent constraints", func(t *testing.T) {
			t.Parallel()
			validator, err := CompileBuiltin(ConfigUserSchemaID)
			if err != nil {
				t.Fatal(err)
			}
			document := CloneDocument(defaultDocument())
			document["render"].(map[string]any)["default_output"] = "invalid-output"
			document["server"].(map[string]any)["port"] = float64(0)
			err = validator.Validate(document)
			var validationErr *jsonschema.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("lost structured validation error: %v", err)
			}
			var changedEnum, changedMinimum bool
			pending := []*jsonschema.ValidationError{validationErr}
			for len(pending) > 0 {
				current := pending[len(pending)-1]
				pending = append(pending[:len(pending)-1], current.Causes...)
				switch constraint := current.ErrorKind.(type) {
				case *kind.Enum:
					for index := range constraint.Want {
						constraint.Want[index] = "invalid-output"
					}
					changedEnum = true
				case *kind.Minimum:
					constraint.Want.SetInt64(-1)
					changedMinimum = true
				}
			}
			if !changedEnum || !changedMinimum {
				t.Fatal("missing mutable enum or minimum constraints in error")
			}
			if err := validator.Validate(CloneDocument(defaultDocument())); err != nil {
				t.Fatalf("returned error changed a valid configuration's result: %v", err)
			}
			if err := validator.Validate(document); err == nil {
				t.Fatal("returned error changed invalid constraints into accepted values")
			}
		})
	}
}

func TestCompileJSONDoesNotReuseCallerSchemaByName(t *testing.T) {
	for _, schema := range []string{`{"type":"string"}`, `{"type":"number"}`} {
		validator, err := CompileJSON(ConfigUserSchemaID, []byte(schema))
		if err != nil {
			t.Fatal(err)
		}
		acceptString := schema == `{"type":"string"}`
		if err := validator.Validate("value"); (err == nil) != acceptString {
			t.Fatalf("caller-provided schema %s: %v", schema, err)
		}
	}
}

func TestExternalConfigSchemaIsReadAgainAfterRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.schema.json")
	if err := os.WriteFile(path, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := NormalizeDocument("user.yaml", path, defaultDocument()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := NormalizeDocument("user.yaml", path, defaultDocument()); err == nil {
		t.Fatal("reused an external schema after its contents changed")
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
