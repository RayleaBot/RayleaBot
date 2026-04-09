package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type pluginInfoFixture struct {
	Input  any `json:"input"`
	Expect struct {
		Valid bool `json:"valid"`
	} `json:"expect"`
}

func TestPluginDiscoveryContextUsesPluginDirectoriesOnly(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	application, err := app.New(app.Options{
		ConfigPath: configPath,
		SchemaPath: filepath.Join("..", "contracts", "config.user.schema.json"),
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	if _, ok := application.Plugins().Get("raylea.help"); !ok {
		t.Fatal("expected builtin plugin to be discovered")
	}
	if _, ok := application.Plugins().Get("hello-python"); ok {
		t.Fatal("examples/plugins must not be discovered by the default application roots")
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
	if snapshot.Role != "builtin" {
		t.Fatalf("unexpected role: got %q want builtin", snapshot.Role)
	}
	if len(snapshot.Commands) != 1 {
		t.Fatalf("unexpected builtin command count: got %d want 1", len(snapshot.Commands))
	}
	if snapshot.Commands[0].Name != "help" {
		t.Fatalf("unexpected builtin command name: got %q want help", snapshot.Commands[0].Name)
	}
}

func TestDiscoverManifestDefaultConfigAndRole(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, filepath.Join("..", "fixtures", "plugin-info", "ok.plugin-with-commands.json"))
	input, ok := fixture.Input.(map[string]any)
	if !ok {
		t.Fatalf("fixture input should be an object, got %T", fixture.Input)
	}
	commands, ok := input["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("fixture commands should be present, got %#v", input["commands"])
	}
	firstCommand, ok := commands[0].(map[string]any)
	if !ok {
		t.Fatalf("first command should be an object, got %#v", commands[0])
	}
	firstCommand["aliases"] = []any{"weather_cn", "tq"}
	pluginDir := filepath.Join(rootDir, "plugins", "weather")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(rootDir, "plugins"),
		}},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if summary.ValidCount != 1 || len(snapshots) != 1 {
		t.Fatalf("unexpected discovery summary: %#v len=%d", summary, len(snapshots))
	}

	snapshot := snapshots[0]
	if snapshot.Role != "user" {
		t.Fatalf("unexpected role: got %q want user", snapshot.Role)
	}
	if got := snapshot.DefaultConfig["default_city"]; got != "北京" {
		t.Fatalf("unexpected default_config.default_city: got %#v want 北京", got)
	}
	if got := snapshot.DefaultConfig["unit"]; got != "celsius" {
		t.Fatalf("unexpected default_config.unit: got %#v want celsius", got)
	}
}

func TestDiscoverManifestWebhookScopes(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, filepath.Join("..", "fixtures", "plugin-info", "ok.minimal-python.json"))
	input, ok := fixture.Input.(map[string]any)
	if !ok {
		t.Fatalf("fixture input should be an object, got %T", fixture.Input)
	}
	input["capabilities"] = []any{"event.subscribe", "event.expose_webhook"}
	input["permissions"] = map[string]any{
		"required": []any{"event.expose_webhook"},
		"optional": []any{},
		"scopes": map[string]any{
			"webhooks": []any{
				map[string]any{
					"route":         "github",
					"auth_strategy": "hmac_sha256",
					"header":        "X-Hub-Signature-256",
					"secret_ref":    "webhook.github.secret",
					"source_ips":    []any{"192.0.2.0/24"},
				},
			},
		},
	}

	pluginDir := filepath.Join(rootDir, "plugins", "repo-watcher")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots: []plugins.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(rootDir, "plugins"),
		}},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d want 1", len(snapshots))
	}

	snapshot := snapshots[0]
	if len(snapshot.ScopeWebhooks) != 1 {
		t.Fatalf("unexpected webhook scope count: %#v", snapshot.ScopeWebhooks)
	}
	scope := snapshot.ScopeWebhooks[0]
	if scope.Route != "github" || scope.AuthStrategy != "hmac_sha256" || scope.SecretRef != "webhook.github.secret" {
		t.Fatalf("unexpected webhook scope: %#v", scope)
	}
	if len(scope.SourceIPs) != 1 || scope.SourceIPs[0] != "192.0.2.0/24" {
		t.Fatalf("unexpected webhook scope source IPs: %#v", scope.SourceIPs)
	}
}
