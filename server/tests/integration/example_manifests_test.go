package integration

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestExamplePluginManifestsMatchContract(t *testing.T) {
	t.Parallel()

	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	manifestPaths, err := filepath.Glob(testutil.RepoPath(t, "examples", "plugins", "*", "info.json"))
	if err != nil {
		t.Fatalf("glob example plugin manifests: %v", err)
	}
	if len(manifestPaths) == 0 {
		t.Fatal("no example plugin manifests found")
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

func compileSchema(t *testing.T, path string) *config.Validator {
	t.Helper()

	validator, err := config.Compile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}

	return validator
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	document, err := config.LoadJSONFile(path)
	if err != nil {
		t.Fatalf("load json %s: %v", path, err)
	}

	return document
}
