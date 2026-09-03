package catalog_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestDiscoverProjectsManifestV3(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "subscription-hub")
	manifest := baseManifest("subscription-hub")
	manifest["events"] = []string{"message.group", "message.private"}
	manifest["permissions"] = map[string]any{
		"message.send":            true,
		"thirdparty.account.read": map[string]any{"platforms": []string{"bilibili", "weibo"}},
	}
	manifest["default_config"] = map[string]any{"help_commands": []string{"解析帮助", "链接帮助"}}
	manifest["commands"] = []any{
		map[string]any{
			"id": "status", "name": "订阅状态", "description": "查看订阅状态", "usage": "/订阅状态",
			"permission": "everyone", "trigger": map[string]any{"type": "exact", "names": []string{"订阅状态", "推送状态"}},
		},
		map[string]any{
			"id": "toggle", "name": "解析开关", "description": "切换解析", "usage": "/开启B站解析",
			"permission": "super_admin", "trigger": map[string]any{"type": "pattern", "pattern": "^(开启|关闭)(B站|微博|抖音)解析$"},
		},
		map[string]any{
			"id": "help", "name": "解析帮助", "description": "查看解析帮助", "usage": "/解析帮助",
			"permission": "everyone", "trigger": map[string]any{"type": "setting", "settings_key": "help_commands"},
		},
	}
	manifest["command_groups"] = []any{
		map[string]any{"id": "subscription", "title": "订阅管理", "commands": []string{"status"}},
		map[string]any{"id": "resolver", "title": "解析管理", "commands": []string{"toggle", "help"}},
	}
	manifest["help"] = map[string]any{"title": "订阅与解析", "summary": "管理订阅推送与链接解析"}
	writeArtifact(t, pluginRoot, manifest, nil)

	snapshot := discoverOne(t, root)
	if !snapshot.Valid || snapshot.ManifestVersion != "3" || snapshot.ArtifactVersion != "2" {
		t.Fatalf("unexpected contract projection: %#v", snapshot)
	}
	if len(snapshot.Events) != 2 || len(snapshot.Permissions) != 2 || len(snapshot.CommandGroups) != 2 {
		t.Fatalf("manifest collections were not projected: %#v", snapshot)
	}
	if got := snapshot.Permissions["thirdparty.account.read"].Platforms; len(got) != 2 || got[0] != "bilibili" || got[1] != "weibo" {
		t.Fatalf("platform permission = %#v", got)
	}
	if len(snapshot.Commands) != 3 {
		t.Fatalf("commands = %#v", snapshot.Commands)
	}
	if snapshot.Commands[0].ID != "status" || snapshot.Commands[0].Name != "订阅状态" || len(snapshot.Commands[0].Aliases) != 1 {
		t.Fatalf("exact command projection = %#v", snapshot.Commands[0])
	}
	if snapshot.Commands[1].MatchPattern == "" || snapshot.Commands[2].Name != "解析帮助" || len(snapshot.Commands[2].Aliases) != 1 {
		t.Fatalf("pattern/setting command projection = %#v", snapshot.Commands)
	}
	if snapshot.Help == nil || snapshot.Help.Title != "订阅与解析" || snapshot.Help.Summary == "" {
		t.Fatalf("help = %#v", snapshot.Help)
	}
}

func TestDiscoverProjectsSingleManagementEntryAndStaticWebhook(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "control-panel")
	manifest := baseManifest("control-panel")
	manifest["events"] = []string{"webhook.received"}
	manifest["management_ui"] = map[string]any{
		"entry": "ui/index.html",
		"pages": []any{
			map[string]any{"id": "overview", "label": "概览"},
			map[string]any{"id": "settings", "label": "设置"},
		},
	}
	manifest["webhooks"] = []any{map[string]any{
		"id": "updates", "route": "updates", "auth_strategy": "hmac_sha256",
		"header": "X-Raylea-Signature", "secret_ref": "webhook.signing_key", "signature_prefix": "sha256=",
		"source_cidrs": []string{"192.0.2.0/24"}, "max_body_bytes": 1048576,
		"replay_protection": map[string]any{
			"timestamp_header": "X-Raylea-Timestamp", "event_id_header": "X-Raylea-Event-ID",
			"tolerance_seconds": 300, "enforce": true,
		},
	}}
	writeArtifact(t, pluginRoot, manifest, map[string][]byte{"ui/index.html": []byte("<!doctype html><title>Control</title>")})

	snapshot := discoverOne(t, root)
	if snapshot.ManagementUI == nil || snapshot.ManagementUI.Entry != "ui/index.html" || len(snapshot.ManagementUI.Pages) != 2 {
		t.Fatalf("management UI = %#v", snapshot.ManagementUI)
	}
	if !snapshot.ArtifactUIAvailable {
		t.Fatalf("management entry was not recognized as an artifact UI: %#v", snapshot)
	}
	if len(snapshot.Webhooks) != 1 {
		t.Fatalf("webhooks = %#v", snapshot.Webhooks)
	}
	webhook := snapshot.Webhooks[0]
	if webhook.ID != "updates" || webhook.Route != "updates" || webhook.SourceCIDRs[0] != "192.0.2.0/24" || webhook.MaxBodyBytes != 1048576 {
		t.Fatalf("webhook = %#v", webhook)
	}
	if !webhook.ReplayProtection.Enforce || webhook.ReplayProtection.ToleranceSeconds != 300 {
		t.Fatalf("replay protection = %#v", webhook.ReplayProtection)
	}
}

func TestDiscoverAutoDiscoversRenderTemplates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "weather-card")
	writeArtifact(t, pluginRoot, baseManifest("weather-card"), map[string][]byte{
		"templates/weather/template.json": []byte(`{"id":"weather","version":"1"}`),
		"templates/weather/template.html": []byte("<html></html>"),
	})

	snapshot := discoverOne(t, root)
	if len(snapshot.RenderTemplates) != 1 || snapshot.RenderTemplates[0].Path != "templates/weather" {
		t.Fatalf("render templates = %#v", snapshot.RenderTemplates)
	}
}

func TestDiscoverKeepsUnsupportedManifestVisibleAndDisabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "legacy")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"id": "legacy", "name": "Legacy", "version": "0.3.0", "manifest_version": "2",
		"plugin_protocol_version": "1", "runtime": "go", "entry": "bin/legacy", "license": "MIT",
	}
	writeJSON(t, filepath.Join(pluginRoot, "info.json"), legacy)

	snapshot := discoverOne(t, root)
	if snapshot.Valid || snapshot.DisplayState != plugins.DisplayStateInvalidManifest || snapshot.DesiredState != plugins.DesiredStateDisabled {
		t.Fatalf("legacy snapshot = %#v", snapshot)
	}
	if snapshot.PluginID != "legacy" || strings.TrimSpace(snapshot.ValidationSummary) == "" {
		t.Fatalf("legacy identity/reason was not preserved: %#v", snapshot)
	}
}

func TestDiscoverMarksBrokenArtifactEntryInvalid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "tampered")
	writeArtifact(t, pluginRoot, baseManifest("tampered"), nil)
	entry := artifactEntry(t, "tampered")
	if err := os.WriteFile(filepath.Join(pluginRoot, filepath.FromSlash(entry)), []byte("not an executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	snapshot := discoverOne(t, root)
	if snapshot.Valid || snapshot.DisplayState != plugins.DisplayStateInvalidManifest || strings.TrimSpace(snapshot.ValidationSummary) == "" {
		t.Fatalf("broken snapshot = %#v", snapshot)
	}
}

func TestDiscoverConflictPathsUseStableSourceOrdering(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, source := range []string{"b", "a"} {
		writeArtifact(t, filepath.Join(root, source, "duplicate"), baseManifest("duplicate"), nil)
	}
	validator := compileSchema(t)
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: validator,
		Roots: []plugincatalog.ScanRoot{
			{Label: "b", Path: filepath.Join(root, "b")},
			{Label: "a", Path: filepath.Join(root, "a")},
		},
		RepoRoot: root,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(snapshots) != 1 || len(snapshots[0].ConflictPaths) != 2 || snapshots[0].DisplayState != plugins.DisplayStateConflict {
		t.Fatalf("conflict snapshot = %#v", snapshots)
	}
	if snapshots[0].ConflictPaths[0] > snapshots[0].ConflictPaths[1] {
		t.Fatalf("conflict paths are not stable: %#v", snapshots[0].ConflictPaths)
	}
}

func discoverOne(t *testing.T, repoRoot string) plugins.Snapshot {
	t.Helper()
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: compileSchema(t),
		Roots:     []plugincatalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Join(repoRoot, "plugins", "installed")}},
		RepoRoot:  repoRoot,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1: %#v", len(snapshots), snapshots)
	}
	return snapshots[0]
}

func compileSchema(t *testing.T) *config.Validator {
	t.Helper()
	validator, err := config.Compile(testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin manifest schema: %v", err)
	}
	return validator
}

func baseManifest(pluginID string) map[string]any {
	return map[string]any{
		"id": pluginID, "name": pluginID, "version": "0.4.0", "manifest_version": "3",
		"license": "MIT", "min_core_version": "0.4.0",
		"metadata": map[string]any{"description": "fixture plugin", "author": "raylea"},
		"events":   []string{}, "permissions": map[string]any{},
	}
}

func writeArtifact(t *testing.T, root string, manifest map[string]any, assets map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "info.json"), manifest)
	for relative, payload := range assets {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pluginID, _ := manifest["id"].(string)
	entry := artifactEntry(t, pluginID)
	entryPath := filepath.Join(root, filepath.FromSlash(entry))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyFile(t, executable, entryPath)

	writeJSON(t, filepath.Join(root, "artifact.json"), map[string]any{
		"artifact_version": "2", "target_platform": currentPlatform(t), "entry": entry,
	})
}

func artifactEntry(t *testing.T, pluginID string) string {
	t.Helper()
	entry := "bin/" + pluginID
	if currentPlatform(t) == "windows-x64" {
		entry += ".exe"
	}
	return entry
}

func currentPlatform(t *testing.T) string {
	t.Helper()
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	return platform
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
