package discovery_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/testapp"
	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
)

type pluginInfoFixture struct {
	Input  any `json:"input"`
	Expect struct {
		Valid bool `json:"valid"`
	} `json:"expect"`
}

func writePersistentYAMLConfig(t *testing.T, databasePath string) string {
	return testapp.WritePersistentYAMLConfig(t, databasePath)
}

func compileSchema(t *testing.T, path string) *schema.Validator {
	t.Helper()

	validator, err := schema.Compile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}

	return validator
}

func TestPluginDiscoveryContextUsesPluginDirectoriesOnly(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	repoRoot := t.TempDir()
	builtinRoot := filepath.Join(repoRoot, "plugins", "builtin")
	exampleRoot := filepath.Join(repoRoot, "examples", "plugins", "hello-python")
	for _, dir := range []string{
		filepath.Join(builtinRoot, "fixture-builtin"),
		exampleRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := writePluginManifest(filepath.Join(builtinRoot, "fixture-builtin", "info.json"), pluginManifestWithCommand("fixture-builtin", "fixture")); err != nil {
		t.Fatalf("write builtin manifest: %v", err)
	}
	if err := writePluginManifest(filepath.Join(exampleRoot, "info.json"), pluginManifestWithCommand("fixture-example", "example")); err != nil {
		t.Fatalf("write example manifest: %v", err)
	}

	application, err := app.New(app.Options{
		ConfigPath:       configPath,
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugindiscovery.ScanRoot{
			{Label: "plugins/builtin", Path: builtinRoot},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	if _, ok := application.Plugins().Get("fixture-builtin"); !ok {
		t.Fatal("expected plugin from configured builtin root to be discovered")
	}
	if _, ok := application.Plugins().Get("fixture-example"); ok {
		t.Fatal("examples/plugins must not be discovered by the default application roots")
	}
}

func TestDefaultAppStartupDoesNotRequireContractsDirectory(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("schema_version: \"2\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(repoRoot, "plugins", "builtin"),
		filepath.Join(repoRoot, "plugins", "installed"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create plugin root %s: %v", dir, err)
		}
	}

	application, err := app.New(app.Options{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("app.New without contracts directory failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})
}

func TestDiscoverInvalidManifestFromFixture(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "invalid.unsupported-binary-runtime.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "invalid-binary")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{
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
	if snapshot.PluginID != "unsupported-binary-tool" {
		t.Fatalf("unexpected plugin_id: got %q want unsupported-binary-tool", snapshot.PluginID)
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
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.minimal-python.json"))

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

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{
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
	if snapshot.ValidationSummary != "多个目录中发现相同插件 ID" {
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

func TestConflictPathsUseStableSourceOrdering(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.minimal-python.json"))

	for _, dir := range []string{"b", "a"} {
		pluginDir := filepath.Join(rootDir, "plugins", dir)
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pluginDir, err)
		}
		if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
			t.Fatalf("write manifest in %s: %v", pluginDir, err)
		}
	}

	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{
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

func TestDiscoverBuiltinSourceDefaultsToEnabledAndPreservesCommands(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	builtinRoot := filepath.Join(repoRoot, "plugins", "builtin")
	pluginDir := filepath.Join(builtinRoot, "fixture-builtin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), pluginManifestWithCommand("fixture-builtin", "fixture")); err != nil {
		t.Fatalf("write builtin manifest: %v", err)
	}

	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{
			{
				Label: "plugins/builtin",
				Path:  builtinRoot,
			},
		},
		RepoRoot: repoRoot,
	})
	if err != nil {
		t.Fatalf("Discover builtin source failed: %v", err)
	}

	catalog := plugincatalog.New(snapshots)
	for _, tc := range []struct {
		pluginID      string
		commandName   string
		source        string
		declarationID string
		commandCount  int
	}{
		{pluginID: "fixture-builtin", commandName: "fixture", source: plugins.CommandSourceManifest, commandCount: 1},
	} {
		snapshot, ok := catalog.Get(tc.pluginID)
		if !ok {
			t.Fatalf("expected plugin %q to be discovered", tc.pluginID)
		}
		if snapshot.DesiredState != "enabled" {
			t.Fatalf("unexpected desired_state for %s: got %q want enabled", tc.pluginID, snapshot.DesiredState)
		}
		if snapshot.Role != "builtin" {
			t.Fatalf("unexpected role for %s: got %q want builtin", tc.pluginID, snapshot.Role)
		}
		if len(snapshot.Commands) != tc.commandCount {
			t.Fatalf("unexpected builtin command count for %s: got %d want %d", tc.pluginID, len(snapshot.Commands), tc.commandCount)
		}
		command, ok := findPluginCommand(snapshot.Commands, tc.commandName)
		if !ok {
			t.Fatalf("expected builtin command %q for %s, got %#v", tc.commandName, tc.pluginID, snapshot.Commands)
		}
		if command.CommandSource != tc.source {
			t.Fatalf("unexpected builtin command source for %s: got %q want %q", tc.pluginID, command.CommandSource, tc.source)
		}
		if command.DeclarationID != tc.declarationID {
			t.Fatalf("unexpected builtin command declaration for %s: got %q want %q", tc.pluginID, command.DeclarationID, tc.declarationID)
		}
	}
}

func pluginManifestWithCommand(pluginID string, commandName string) map[string]any {
	return map[string]any{
		"id":                      pluginID,
		"name":                    pluginID,
		"version":                 "0.1.0",
		"manifest_version":        "1",
		"plugin_protocol_version": "1",
		"type":                    "managed_runtime",
		"runtime":                 "python",
		"entry":                   "main.py",
		"license":                 "MIT",
		"capabilities":            []any{"event.subscribe", "message.send"},
		"commands": []any{
			map[string]any{
				"name":        commandName,
				"description": "fixture command",
				"usage":       "/" + commandName,
				"permission":  "everyone",
			},
		},
	}
}

func findPluginCommand(commands []plugins.Command, name string) (plugins.Command, bool) {
	for _, command := range commands {
		if command.Name == name {
			return command, true
		}
	}
	return plugins.Command{}, false
}

func TestDiscoverManifestDynamicCommands(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.plugin-with-dynamic-commands.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "fortune")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if len(snapshot.DynamicCommands) != 1 {
		t.Fatalf("dynamic command declarations = %#v, want one", snapshot.DynamicCommands)
	}
	if len(snapshot.Commands) != 1 {
		t.Fatalf("projected commands = %#v, want one", snapshot.Commands)
	}
	command := snapshot.Commands[0]
	if command.Name != "我的运势" || !reflect.DeepEqual(command.Aliases, []string{"今日运势"}) {
		t.Fatalf("unexpected projected dynamic command: %#v", command)
	}
	if command.CommandSource != plugins.CommandSourceDynamic || command.DeclarationID != "fortune" || command.Permission != "everyone" {
		t.Fatalf("unexpected dynamic command metadata: %#v", command)
	}
}

func TestDiscoverManifestCommandPatterns(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.plugin-with-command-patterns.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "game-guide")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if len(snapshot.CommandPatterns) != 1 {
		t.Fatalf("command pattern declarations = %#v, want one", snapshot.CommandPatterns)
	}
	if len(snapshot.Commands) != 1 {
		t.Fatalf("projected commands = %#v, want one", snapshot.Commands)
	}
	command := snapshot.Commands[0]
	if command.Name != "角色攻略" || command.MatchPattern != "^(.+?)攻略$" {
		t.Fatalf("unexpected projected pattern command: %#v", command)
	}
	if command.CommandSource != plugins.CommandSourcePattern || command.DeclarationID != "character-guide" || command.Permission != "everyone" {
		t.Fatalf("unexpected pattern command metadata: %#v", command)
	}
}

func TestDiscoverManifestRejectsInvalidCommandPattern(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.plugin-with-command-patterns.json"))
	input := fixture.Input.(map[string]any)
	patterns := input["command_patterns"].([]any)
	patterns[0].(map[string]any)["pattern"] = "["

	pluginDir := filepath.Join(rootDir, "plugins", "game-guide")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(rootDir, "plugins"),
		}},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if summary.InvalidCount != 1 || len(snapshots) != 1 {
		t.Fatalf("unexpected discovery summary: %#v len=%d", summary, len(snapshots))
	}
	if snapshots[0].Valid || !strings.Contains(snapshots[0].ValidationSummary, "command_patterns[0].pattern is invalid") {
		t.Fatalf("unexpected invalid pattern snapshot: %#v", snapshots[0])
	}
}

func TestDiscoverManifestDefaultConfigFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.default-config-file.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "weather-file-config")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	defaultConfig := map[string]any{
		"trigger_commands": []any{"weather", "forecast"},
		"unit":             "metric",
		"default_city":     "上海",
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "defaults.json"), defaultConfig); err != nil {
		t.Fatalf("write default config: %v", err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if got := snapshot.DefaultConfig["default_city"]; got != "上海" {
		t.Fatalf("unexpected default_config.default_city: got %#v want 上海", got)
	}
	if got := snapshot.DefaultConfig["unit"]; got != "celsius" {
		t.Fatalf("unexpected default_config.unit: got %#v want celsius", got)
	}
	if len(snapshot.Commands) != 1 || snapshot.Commands[0].Name != "weather" || !reflect.DeepEqual(snapshot.Commands[0].Aliases, []string{"forecast"}) {
		t.Fatalf("unexpected commands from default_config_file: %#v", snapshot.Commands)
	}
}

func TestDiscoverManifestDefaultConfigAndRole(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.plugin-with-commands.json"))
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

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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

func TestDiscoverManifestManagementUI(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.management-ui.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "example-config-panel")
	if err := os.MkdirAll(filepath.Join(pluginDir, "web"), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if snapshot.ManagementUI == nil {
		t.Fatal("expected management_ui to be populated")
	}
	if len(snapshot.ManagementUI.Pages) != 1 {
		t.Fatalf("unexpected management_ui.pages length: got %d want 1", len(snapshot.ManagementUI.Pages))
	}
	if got := snapshot.ManagementUI.Pages[0]; got.ID != "config" || got.Label != "配置页面" || got.Entry != "web/index.html" {
		t.Fatalf("unexpected management_ui page: %#v", got)
	}
	if snapshot.PackageRootPath != pluginDir {
		t.Fatalf("unexpected package root path: got %q want %q", snapshot.PackageRootPath, pluginDir)
	}
}

func TestDiscoverManifestManagementUIPages(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.management-ui-pages.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "example-config-panel")
	if err := os.MkdirAll(filepath.Join(pluginDir, "web"), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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

	pages := snapshots[0].ManagementUI.Pages
	if len(pages) != 2 {
		t.Fatalf("unexpected management_ui.pages length: got %d want 2", len(pages))
	}
	if pages[0].ID != "config" || pages[0].Entry != "web/index.html" {
		t.Fatalf("unexpected first management page: %#v", pages[0])
	}
	if pages[1].ID != "secrets" || pages[1].Entry != "web/secrets.html" {
		t.Fatalf("unexpected second management page: %#v", pages[1])
	}
}

func TestDiscoverManifestRejectsManagementUIPageOutsideEntryDirectory(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.management-ui-pages.json"))
	input := fixture.Input.(map[string]any)
	managementUI := input["management_ui"].(map[string]any)
	pages := managementUI["pages"].([]any)
	secondPage := pages[1].(map[string]any)
	secondPage["entry"] = "admin/secrets.html"

	pluginDir := filepath.Join(rootDir, "plugins", "example-config-panel")
	if err := os.MkdirAll(filepath.Join(pluginDir, "web"), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(rootDir, "plugins"),
		}},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if summary.InvalidCount != 1 || len(snapshots) != 1 {
		t.Fatalf("unexpected discovery summary: %#v len=%d", summary, len(snapshots))
	}
	if snapshots[0].Valid {
		t.Fatal("expected invalid snapshot")
	}
	if !strings.Contains(snapshots[0].ValidationSummary, "must stay inside") {
		t.Fatalf("unexpected validation summary: %q", snapshots[0].ValidationSummary)
	}
}

func TestDiscoverManifestRejectsDuplicateManagementUIPageID(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.management-ui-pages.json"))
	input := fixture.Input.(map[string]any)
	managementUI := input["management_ui"].(map[string]any)
	pages := managementUI["pages"].([]any)
	secondPage := pages[1].(map[string]any)
	secondPage["id"] = "config"

	pluginDir := filepath.Join(rootDir, "plugins", "example-config-panel")
	if err := os.MkdirAll(filepath.Join(pluginDir, "web"), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(rootDir, "plugins"),
		}},
		RepoRoot: rootDir,
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if summary.InvalidCount != 1 || len(snapshots) != 1 {
		t.Fatalf("unexpected discovery summary: %#v len=%d", summary, len(snapshots))
	}
	if snapshots[0].Valid {
		t.Fatal("expected invalid snapshot")
	}
	if !strings.Contains(snapshots[0].ValidationSummary, "duplicate id") {
		t.Fatalf("unexpected validation summary: %q", snapshots[0].ValidationSummary)
	}
}

func TestDiscoverManifestRenderTemplates(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.render-template.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "weather-card")
	if err := os.MkdirAll(filepath.Join(pluginDir, "templates", "card"), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if len(snapshot.RenderTemplates) != 1 || snapshot.RenderTemplates[0].Path != "templates/card" {
		t.Fatalf("unexpected render_templates: %#v", snapshot.RenderTemplates)
	}
}

func TestDiscoverManifestRichMetadata(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.rich-metadata.json"))
	pluginDir := filepath.Join(rootDir, "plugins", "weather-rich")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pluginDir, err)
	}
	if err := writePluginManifest(filepath.Join(pluginDir, "info.json"), fixture.Input); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	snapshots, summary, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
	if snapshot.Author != "raylea" {
		t.Fatalf("unexpected author: got %q want raylea", snapshot.Author)
	}
	if snapshot.License != "MIT" {
		t.Fatalf("unexpected license: got %q want MIT", snapshot.License)
	}
	if snapshot.Icon != "assets/weather.svg" {
		t.Fatalf("unexpected icon: got %q want assets/weather.svg", snapshot.Icon)
	}
	if snapshot.Repo != "https://github.com/RayleaBot/plugins-weather" {
		t.Fatalf("unexpected repo: got %q", snapshot.Repo)
	}
	if snapshot.Homepage != "https://plugins.rayleabot.local/weather" {
		t.Fatalf("unexpected homepage: got %q", snapshot.Homepage)
	}
	if !reflect.DeepEqual(snapshot.Keywords, []string{"weather", "forecast", "climate"}) {
		t.Fatalf("unexpected keywords: %#v", snapshot.Keywords)
	}
	if len(snapshot.Screenshots) != 1 || snapshot.Screenshots[0].Path != "assets/overview.svg" || snapshot.Screenshots[0].Alt != "天气总览卡片" {
		t.Fatalf("unexpected screenshots: %#v", snapshot.Screenshots)
	}
	if !reflect.DeepEqual(snapshot.SystemDependencies, []string{"OneBot11 connection", "External weather API access"}) {
		t.Fatalf("unexpected system dependencies: %#v", snapshot.SystemDependencies)
	}
	if snapshot.Concurrency != 3 {
		t.Fatalf("unexpected concurrency: got %d want 3", snapshot.Concurrency)
	}
	if got := snapshot.DefaultConfig["forecast_days"]; got != float64(3) {
		t.Fatalf("unexpected default_config.forecast_days: got %#v want 3", got)
	}
}

func TestDiscoverManifestWebhookScopes(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	fixture := loadPluginInfoFixture(t, testutil.RepoPath(t, "fixtures", "plugin-info", "ok.minimal-python.json"))
	input, ok := fixture.Input.(map[string]any)
	if !ok {
		t.Fatalf("fixture input should be an object, got %T", fixture.Input)
	}
	input["capabilities"] = []any{"event.subscribe", "event.expose_webhook"}
	input["capability_parameters"] = map[string]any{
		"webhooks": []any{
			map[string]any{
				"route":         "github",
				"auth_strategy": "hmac_sha256",
				"header":        "X-Hub-Signature-256",
				"secret_ref":    "webhook.github.secret",
				"source_ips":    []any{"192.0.2.0/24"},
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

	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
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
