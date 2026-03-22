package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rayleabot/server/internal/plugins"
)

type pluginInfoFixture struct {
	Input  any `json:"input"`
	Expect struct {
		Valid bool `json:"valid"`
	} `json:"expect"`
}

func TestDiscoverExamplesPlugins(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootPath(t)
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	snapshots, summary, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: "examples/plugins",
				Path:  filepath.Join(repoRoot, "examples", "plugins"),
			},
		},
		RepoRoot: repoRoot,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if summary.ValidCount != 4 {
		t.Fatalf("unexpected valid count: got %d want 4", summary.ValidCount)
	}

	catalog := plugins.NewCatalog(snapshots)
	for _, pluginID := range []string{"echo-python", "hello-node", "hello-python", "notice-logger"} {
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			t.Fatalf("expected plugin %s to be discovered", pluginID)
		}
		if !snapshot.Valid {
			t.Fatalf("expected plugin %s to be valid", pluginID)
		}
		if snapshot.RegistrationState != "installed" {
			t.Fatalf("unexpected registration_state for %s: %s", pluginID, snapshot.RegistrationState)
		}
		if snapshot.DesiredState != "disabled" {
			t.Fatalf("unexpected desired_state for %s: %s", pluginID, snapshot.DesiredState)
		}
		if snapshot.RuntimeState != "stopped" {
			t.Fatalf("unexpected runtime_state for %s: %s", pluginID, snapshot.RuntimeState)
		}
		if snapshot.DisplayState != "discovered" {
			t.Fatalf("unexpected display_state for %s: %s", pluginID, snapshot.DisplayState)
		}
	}
}

func TestDiscoverInvalidManifestFromFixture(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, filepath.Join("..", "fixtures", "plugin-info", "invalid.unsupported-binary-runtime.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "invalid-binary")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: "plugins/installed",
				Path:  filepath.Join(rootDir, "plugins"),
			},
		},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if summary.InvalidCount != 1 {
		t.Fatalf("unexpected invalid count: got %d want 1", summary.InvalidCount)
	}
	if len(snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d want 1", len(snapshots))
	}

	snapshot := snapshots[0]
	if snapshot.PluginID != "legacy-binary-tool" {
		t.Fatalf("unexpected plugin_id: got %q want legacy-binary-tool", snapshot.PluginID)
	}
	if snapshot.Valid {
		t.Fatal("expected invalid snapshot")
	}
	if snapshot.DisplayState != "invalid_manifest" {
		t.Fatalf("unexpected display_state: got %q want invalid_manifest", snapshot.DisplayState)
	}
	if snapshot.ValidationSummary == "" {
		t.Fatal("expected validation summary to be populated")
	}
}

func TestDiscoverPluginIDConflict(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, filepath.Join("..", "fixtures", "plugin-info", "ok.minimal-python.json"))

	firstDir := filepath.Join(rootDir, "plugins", "weather-a")
	secondDir := filepath.Join(rootDir, "plugins", "weather-b")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := writePluginManifest(filepath.Join(dir, "info.json"), fixture.Input); err != nil {
			t.Fatalf("write manifest in %s: %v", dir, err)
		}
	}

	snapshots, summary, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: "plugins/installed",
				Path:  filepath.Join(rootDir, "plugins"),
			},
		},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if summary.ConflictCount != 1 {
		t.Fatalf("unexpected conflict count: got %d want 1", summary.ConflictCount)
	}
	if len(snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d want 1", len(snapshots))
	}

	snapshot := snapshots[0]
	if snapshot.PluginID != "weather" {
		t.Fatalf("unexpected plugin_id: got %q want weather", snapshot.PluginID)
	}
	if snapshot.DisplayState != "conflict" {
		t.Fatalf("unexpected display_state: got %q want conflict", snapshot.DisplayState)
	}
	if len(snapshot.ConflictPaths) != 2 {
		t.Fatalf("unexpected conflict path count: got %d want 2", len(snapshot.ConflictPaths))
	}
	if snapshot.ValidationSummary != "duplicate plugin_id discovered across multiple directories" {
		t.Fatalf("unexpected validation summary: %q", snapshot.ValidationSummary)
	}
}

func loadPluginInfoFixture(t *testing.T, path string) pluginInfoFixture {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture pluginInfoFixture
	if err := json.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}

	return fixture
}

func writePluginManifest(path string, document any) error {
	bytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bytes, 0o644)
}

func repoRootPath(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	return root
}

func TestConflictPathsUseStableSourceOrdering(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, filepath.Join("..", "fixtures", "plugin-info", "ok.minimal-python.json"))

	for _, dir := range []string{"b", "a"} {
		pluginDir := filepath.Join(rootDir, "plugins", dir)
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pluginDir, err)
		}
		if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
			t.Fatalf("write manifest in %s: %v", pluginDir, err)
		}
	}

	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: "plugins/installed",
				Path:  filepath.Join(rootDir, "plugins"),
			},
		},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d want 1", len(snapshots))
	}
	if !strings.Contains(strings.Join(snapshots[0].ConflictPaths, ","), "info.json") {
		t.Fatal("expected conflict paths to include manifest filenames")
	}
}

func TestDiscoverBuiltinPluginDefaultsToEnabledAndPreservesCommands(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootPath(t)
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{
			{
				Label: "plugins/builtin",
				Path:  filepath.Join(repoRoot, "plugins", "builtin"),
			},
		},
		RepoRoot: repoRoot,
	})
	if err != nil {
		t.Fatalf("Discover builtin plugins failed: %v", err)
	}

	catalog := plugins.NewCatalog(snapshots)
	snapshot, ok := catalog.Get("raylea.help")
	if !ok {
		t.Fatal("expected builtin help plugin to be discovered")
	}
	if snapshot.DesiredState != "enabled" {
		t.Fatalf("unexpected desired_state: got %q want enabled", snapshot.DesiredState)
	}
	if len(snapshot.Commands) != 1 {
		t.Fatalf("unexpected builtin command count: got %d want 1", len(snapshot.Commands))
	}
	if snapshot.Commands[0].Name != "help" {
		t.Fatalf("unexpected builtin command name: got %q want help", snapshot.Commands[0].Name)
	}
}
