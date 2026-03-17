package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestExamplePluginManifestsMatchContract(t *testing.T) {
	t.Parallel()

	schema := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	manifestPaths := []string{
		filepath.Join("..", "examples", "plugins", "hello-python", "info.json"),
		filepath.Join("..", "examples", "plugins", "hello-node", "info.json"),
	}

	for _, manifestPath := range manifestPaths {
		manifestPath := manifestPath
		t.Run(filepath.Base(filepath.Dir(manifestPath)), func(t *testing.T) {
			t.Parallel()

			document := loadJSONDocument(t, manifestPath)
			if err := schema.Validate(document); err != nil {
				t.Fatalf("schema validation failed for %s: %v", manifestPath, err)
			}
		})
	}
}

func compileSchema(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve schema path %s: %v", path, err)
	}

	document := loadJSONDocument(t, absolutePath)
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(absolutePath, document); err != nil {
		t.Fatalf("add schema resource %s: %v", absolutePath, err)
	}

	schema, err := compiler.Compile(absolutePath)
	if err != nil {
		t.Fatalf("compile schema %s: %v", absolutePath, err)
	}

	return schema
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}

	var document any
	if err := json.Unmarshal(bytes, &document); err != nil {
		t.Fatalf("unmarshal json %s: %v", path, err)
	}

	return document
}
