package server

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func TestBundledPluginManifestsMatchContract(t *testing.T) {
	t.Parallel()

	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	manifestPaths := []string{
		filepath.Join("..", "examples", "plugins", "echo-python", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-config-panel", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-permission-scope", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-render-card", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-scheduler", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-webhook", "info.json"),
		filepath.Join("..", "examples", "plugins", "hello-python", "info.json"),
		filepath.Join("..", "examples", "plugins", "hello-node", "info.json"),
		filepath.Join("..", "examples", "plugins", "notice-logger", "info.json"),
		filepath.Join("..", "plugins", "builtin", "echo", "info.json"),
		filepath.Join("..", "plugins", "builtin", "help", "info.json"),
	}

	for _, manifestPath := range manifestPaths {
		manifestPath := manifestPath
		t.Run(filepath.Base(filepath.Dir(manifestPath)), func(t *testing.T) {
			t.Parallel()

			document := loadJSONDocument(t, manifestPath)
			if err := validator.Validate(document); err != nil {
				t.Fatalf("schema validation failed for %s: %v", manifestPath, err)
			}
		})
	}
}

func compileSchema(t *testing.T, path string) *schema.Validator {
	t.Helper()

	validator, err := schema.Compile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}

	return validator
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	document, err := schema.LoadJSONFile(path)
	if err != nil {
		t.Fatalf("load json %s: %v", path, err)
	}

	return document
}
