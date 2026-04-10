package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func TestBuildSpecFromDiscoveredExamples(t *testing.T) {
	t.Parallel()

	fixtureRepoRoot := runtimeRepoRoot(t)
	repoRoot := t.TempDir()
	writeRuntimeManifestFile(t, repoRoot)
	copyRuntimeDir(t, filepath.Join(fixtureRepoRoot, "examples", "plugins", "hello-python"), filepath.Join(repoRoot, "examples", "plugins", "hello-python"))
	copyRuntimeDir(t, filepath.Join(fixtureRepoRoot, "examples", "plugins", "hello-node"), filepath.Join(repoRoot, "examples", "plugins", "hello-node"))
	writePreparedManagedRuntime(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "python3"))
	writePreparedManagedRuntime(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "pip3"))
	writePreparedManagedRuntime(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "node"))
	writePreparedManagedRuntime(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "npm"))

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

func TestBuildSpecPrefersPluginVirtualenvForPython(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", "hello-python")
	writeRuntimeManifestFile(t, repoRoot)
	writeRuntimePlugin(t, pluginRoot, "hello-python", "python", "main.py")
	venvPython := filepath.Join(pluginRoot, ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o755); err != nil {
		t.Fatalf("mkdir venv: %v", err)
	}
	if err := os.WriteFile(venvPython, []byte("python"), 0o755); err != nil {
		t.Fatalf("write venv python: %v", err)
	}

	spec, err := BuildSpec(plugins.Snapshot{
		PluginID:     "hello-python",
		Valid:        true,
		Runtime:      "python",
		Entry:        "main.py",
		ManifestPath: filepath.Join("plugins", "installed", "hello-python", "info.json"),
	}, repoRoot, minimalRuntimeConfig())
	if err != nil {
		t.Fatalf("BuildSpec failed: %v", err)
	}
	if spec.Command != venvPython {
		t.Fatalf("BuildSpec should prefer plugin .venv python, got %q want %q", spec.Command, venvPython)
	}
}

func TestBuildSpecFallsBackToManagedRuntimeStore(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", "hello-python")
	writeRuntimeManifestFile(t, repoRoot)
	writeRuntimePlugin(t, pluginRoot, "hello-python", "python", "main.py")
	managedPython := filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "python3")
	managedPip := filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "pip3")
	if err := os.MkdirAll(filepath.Dir(managedPython), 0o755); err != nil {
		t.Fatalf("mkdir managed python: %v", err)
	}
	if err := os.WriteFile(managedPython, []byte("python"), 0o755); err != nil {
		t.Fatalf("write managed python: %v", err)
	}
	if err := os.WriteFile(managedPip, []byte("pip"), 0o755); err != nil {
		t.Fatalf("write managed pip: %v", err)
	}

	spec, err := BuildSpec(plugins.Snapshot{
		PluginID:     "hello-python",
		Valid:        true,
		Runtime:      "python",
		Entry:        "main.py",
		ManifestPath: filepath.Join("plugins", "installed", "hello-python", "info.json"),
	}, repoRoot, minimalRuntimeConfig())
	if err != nil {
		t.Fatalf("BuildSpec failed: %v", err)
	}
	if spec.Command != managedPython {
		t.Fatalf("BuildSpec should fall back to managed python, got %q want %q", spec.Command, managedPython)
	}
	if !sameStrings(spec.Env, []string{"PYTHONIOENCODING=UTF-8", "PYTHONUTF8=1", "PYTHONUNBUFFERED=1"}) {
		t.Fatalf("BuildSpec should inject python utf8 env, got %#v", spec.Env)
	}
}

func TestBuildSpecUsesManagedNodeAndInjectsNodeOptions(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", "hello-node")
	writeRuntimeManifestFile(t, repoRoot)
	writeRuntimePlugin(t, pluginRoot, "hello-node", "nodejs", "index.js")
	managedNode := filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "node")
	managedNpm := filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "npm")
	if err := os.MkdirAll(filepath.Dir(managedNode), 0o755); err != nil {
		t.Fatalf("mkdir managed node: %v", err)
	}
	if err := os.WriteFile(managedNode, []byte("node"), 0o755); err != nil {
		t.Fatalf("write managed node: %v", err)
	}
	if err := os.WriteFile(managedNpm, []byte("npm"), 0o755); err != nil {
		t.Fatalf("write managed npm: %v", err)
	}

	cfg := minimalRuntimeConfig()
	cfg.NodeMaxOldSpaceSizeMB = 384
	spec, err := BuildSpec(plugins.Snapshot{
		PluginID:     "hello-node",
		Valid:        true,
		Runtime:      "nodejs",
		Entry:        "index.js",
		ManifestPath: filepath.Join("plugins", "installed", "hello-node", "info.json"),
	}, repoRoot, cfg)
	if err != nil {
		t.Fatalf("BuildSpec failed: %v", err)
	}
	if spec.Command != managedNode {
		t.Fatalf("BuildSpec should use managed node, got %q want %q", spec.Command, managedNode)
	}
	if len(spec.Env) != 1 || spec.Env[0] != "NODE_OPTIONS=--max-old-space-size=384" {
		t.Fatalf("BuildSpec should inject managed NODE_OPTIONS, got %#v", spec.Env)
	}
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

func writeRuntimeManifestFile(t *testing.T, repoRoot string) {
	t.Helper()

	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir deps root: %v", err)
	}
	content := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + depsPlatformForTests() + `",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/install/bin/python3"],
        "pip": ["python/install/bin/pip3"]
      }
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + depsPlatformForTests() + `",
      "sources": [
        {
          "url": "https://example.invalid/node.tar.xz",
          "kind": "upstream"
        }
      ],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "tar.xz",
      "entrypoints": {
        "node": ["node/bin/node"],
        "npm": ["node/bin/npm"]
      }
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
}

func writeRuntimePlugin(t *testing.T, pluginRoot, pluginID, runtimeName, entry string) {
	t.Helper()

	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatalf("mkdir plugin root: %v", err)
	}
	infoPath := filepath.Join(pluginRoot, "info.json")
	if err := os.WriteFile(infoPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write info.json: %v", err)
	}
	entryPath := filepath.Join(pluginRoot, filepath.FromSlash(entry))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir entry dir: %v", err)
	}
	content := "print('placeholder')\n"
	if runtimeName == "nodejs" {
		content = "console.log('placeholder')\n"
	}
	if err := os.WriteFile(entryPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
}

func copyRuntimeDir(t *testing.T, sourceRoot string, targetRoot string) {
	t.Helper()

	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		t.Fatalf("read source dir %s: %v", sourceRoot, err)
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatalf("mkdir target dir %s: %v", targetRoot, err)
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(sourceRoot, entry.Name())
		targetPath := filepath.Join(targetRoot, entry.Name())
		if entry.IsDir() {
			copyRuntimeDir(t, sourcePath, targetPath)
			continue
		}
		payload, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read source file %s: %v", sourcePath, err)
		}
		if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
			t.Fatalf("write target file %s: %v", targetPath, err)
		}
	}
}

func writePreparedManagedRuntime(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir prepared runtime dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("runtime"), 0o755); err != nil {
		t.Fatalf("write prepared runtime: %v", err)
	}
}

func depsPlatformForTests() string {
	switch goruntime.GOOS {
	case "windows":
		if goruntime.GOARCH == "amd64" {
			return "windows-x64"
		}
		return "windows-" + goruntime.GOARCH
	case "darwin":
		if goruntime.GOARCH == "amd64" {
			return "macos-x64"
		}
		return "macos-" + goruntime.GOARCH
	default:
		if goruntime.GOARCH == "amd64" {
			return goruntime.GOOS + "-x64"
		}
		return goruntime.GOOS + "-" + goruntime.GOARCH
	}
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

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
