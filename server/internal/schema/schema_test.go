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
