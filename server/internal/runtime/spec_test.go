package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/schema"
)

func TestBuildSpecFromDiscoveredExamples(t *testing.T) {
	t.Parallel()

	repoRoot := runtimeRepoRoot(t)
	catalog := discoverRuntimeTestCatalog(t, filepath.Join(repoRoot, "examples", "plugins"))

	for _, pluginID := range []string{"hello-python", "hello-node"} {
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			t.Fatalf("expected discovered plugin %q", pluginID)
		}

		spec, err := BuildSpec(snapshot, repoRoot, minimalRuntimeConfig())
		if err != nil {
			t.Fatalf("build spec for %q: %v", pluginID, err)
		}

		if spec.PluginID != pluginID {
			t.Fatalf("unexpected plugin id: got %q want %q", spec.PluginID, pluginID)
		}
		if spec.EntryPath == "" {
			t.Fatalf("expected non-empty entry path for %q", pluginID)
		}
	}
}

func TestBuildSpecRejectsInvalidOrConflictedPlugin(t *testing.T) {
	t.Parallel()

	repoRoot := runtimeRepoRoot(t)

	t.Run("invalid manifest fixture", func(t *testing.T) {
		root := t.TempDir()
		writePluginManifestFromFixture(t, filepath.Join(root, "legacy-binary-tool"), "legacy-binary-tool", filepath.Join(repoRoot, "fixtures", "plugin-info", "invalid.unsupported-binary-runtime.json"))
		catalog := discoverRuntimeTestCatalog(t, root)

		snapshot, ok := catalog.Get("legacy-binary-tool")
		if !ok {
			t.Fatalf("expected invalid plugin to enter catalog")
		}

		_, err := BuildSpec(snapshot, repoRoot, minimalRuntimeConfig())
		assertBuildSpecErrorCode(t, err, codePlatformInvalidRequest)
	})

	t.Run("conflict", func(t *testing.T) {
		root := t.TempDir()
		fixturePath := filepath.Join(repoRoot, "fixtures", "plugin-info", "ok.minimal-python.json")
		writePluginManifestFromFixture(t, filepath.Join(root, "one"), "duplicate-hello", fixturePath)
		writePluginManifestFromFixture(t, filepath.Join(root, "two"), "duplicate-hello", fixturePath)

		catalog := discoverRuntimeTestCatalog(t, root)
		snapshot, ok := catalog.Get("duplicate-hello")
		if !ok {
			t.Fatalf("expected conflict plugin to enter catalog")
		}
		if snapshot.DisplayState != "conflict" {
			t.Fatalf("unexpected display state: got %q want conflict", snapshot.DisplayState)
		}

		_, err := BuildSpec(snapshot, repoRoot, minimalRuntimeConfig())
		assertBuildSpecErrorCode(t, err, codePlatformInvalidRequest)
	})
}

func TestBuildSpecRejectsEntrySymlinkEscapingPluginDir(t *testing.T) {
	t.Parallel()

	repoRoot := runtimeRepoRoot(t)
	root := filepath.Join(t.TempDir(), "hello-python")
	fixturePath := filepath.Join(repoRoot, "fixtures", "plugin-info", "ok.minimal-python.json")
	writePluginManifestFromFixture(t, root, "hello-python", fixturePath)

	entryPath := filepath.Join(root, filepath.FromSlash(loadFixtureInput(t, fixturePath)["entry"].(string)))
	if err := os.Remove(entryPath); err != nil {
		t.Fatalf("remove entry %s: %v", entryPath, err)
	}

	externalDir := t.TempDir()
	externalEntryPath := filepath.Join(externalDir, "external.py")
	if err := os.WriteFile(externalEntryPath, []byte("print('outside')\n"), 0o644); err != nil {
		t.Fatalf("write external entry %s: %v", externalEntryPath, err)
	}

	if err := os.Symlink(externalEntryPath, entryPath); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	catalog := discoverRuntimeTestCatalog(t, filepath.Dir(root))
	snapshot, ok := catalog.Get("hello-python")
	if !ok {
		t.Fatalf("expected discovered plugin")
	}

	_, err := BuildSpec(snapshot, repoRoot, minimalRuntimeConfig())
	assertBuildSpecErrorCode(t, err, codePlatformInvalidRequest)
}

func discoverRuntimeTestCatalog(t *testing.T, root string) *plugins.Catalog {
	t.Helper()

	repoRoot := runtimeRepoRoot(t)
	validator, err := schema.Compile(filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}

	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: filepath.Base(root),
				Path:  root,
			},
		},
		RepoRoot: repoRoot,
	})
	if err != nil {
		t.Fatalf("discover plugins: %v", err)
	}

	return plugins.NewCatalog(snapshots)
}

func writePluginManifestFromFixture(t *testing.T, root string, pluginID string, fixturePath string) {
	t.Helper()

	manifest := loadFixtureInput(t, fixturePath)
	manifest["id"] = pluginID

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", root, err)
	}

	infoPath := filepath.Join(root, "info.json")
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(infoPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest %s: %v", infoPath, err)
	}

	entryValue, ok := manifest["entry"].(string)
	if !ok || entryValue == "" {
		t.Fatalf("fixture %s is missing entry", fixturePath)
	}
	entryPath := filepath.Join(root, filepath.FromSlash(entryValue))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir entry dir %s: %v", filepath.Dir(entryPath), err)
	}
	if err := os.WriteFile(entryPath, []byte("print('placeholder')\n"), 0o644); err != nil {
		t.Fatalf("write entry %s: %v", entryPath, err)
	}
}

func loadFixtureInput(t *testing.T, path string) map[string]any {
	t.Helper()

	document, err := schema.LoadJSONFile(path)
	if err != nil {
		t.Fatalf("load fixture %s: %v", path, err)
	}

	root, ok := document.(map[string]any)
	if !ok {
		t.Fatalf("fixture %s is not an object", path)
	}

	input, ok := root["input"].(map[string]any)
	if !ok {
		t.Fatalf("fixture %s is missing input", path)
	}

	return input
}

func minimalRuntimeConfig() config.RuntimeConfig {
	return config.RuntimeConfig{
		PluginInitTimeoutSeconds:  1,
		PluginInitMaxTotalSeconds: 5,
		ShutdownGraceSeconds:      1,
	}
}

func runtimeRepoRoot(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	return repoRoot
}

func assertBuildSpecErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected build spec error %q, got nil", want)
	}

	runtimeErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *runtime.Error, got %T", err)
	}
	if runtimeErr.Code != want {
		t.Fatalf("unexpected error code: got %q want %q", runtimeErr.Code, want)
	}
}
