package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func TestReloadRefreshesManifestCommandsAndPermissions(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "raylea.subscription-hub",
		Name:              "Subscription Hub",
		Valid:             true,
		SourceRoot:        "plugins/installed",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Permissions:       map[string]plugins.PermissionGrant{"http.request": {}},
		Commands: []plugins.Command{{
			ID: "subscribe-bilibili", DisplayName: "订阅 Bilibili 推送",
			Name: "订阅b站推送", TriggerType: "exact", TriggerNames: []string{"订阅b站推送"},
			Usage: "/订阅b站推送 UID", Permission: "super_admin",
		}},
		Help: &plugins.Help{Title: "订阅中心", Summary: "旧帮助摘要"},
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(t,
		catalog,
		nil,
		pluginruntime.NewRegistry(slog.Default(), pluginruntime.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	app.services.pluginLifecycle.refreshManifest = func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return RefreshPluginManifest(ctx, catalog, nil, pluginID, func() ([]plugins.Snapshot, error) {
			return []plugins.Snapshot{{
				PluginID:          "raylea.subscription-hub",
				Name:              "Subscription Hub",
				Valid:             true,
				SourceRoot:        "plugins/installed",
				RegistrationState: "installed",
				DesiredState:      "enabled",
				RuntimeState:      "stopped",
				Permissions:       map[string]plugins.PermissionGrant{"http.request": {}, "message.send": {}},
				ManifestCommands: []plugins.Command{{
					ID: "subscribe-bilibili", DisplayName: "订阅 Bilibili 推送",
					Name: "订阅b站推送", TriggerType: "exact", TriggerNames: []string{"订阅b站推送"},
					Usage: "/订阅b站推送 UID或昵称", Permission: "super_admin",
				}},
				Commands: []plugins.Command{{
					ID: "subscribe-bilibili", DisplayName: "订阅 Bilibili 推送",
					Name: "订阅b站推送", TriggerType: "exact", TriggerNames: []string{"订阅b站推送"},
					Usage: "/订阅b站推送 UID或昵称", Permission: "super_admin",
				}},
				Help: &plugins.Help{Title: "订阅中心", Summary: "新帮助摘要"},
			}}, nil
		})
	}

	updated, err := app.services.pluginLifecycle.Reload(context.Background(), "raylea.subscription-hub")
	if err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	if updated.RuntimeState != "starting" {
		t.Fatalf("runtime_state = %q, want starting", updated.RuntimeState)
	}
	snapshot, ok := catalog.Get("raylea.subscription-hub")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "enabled" || snapshot.RuntimeState != "starting" {
		t.Fatalf("state = desired %q runtime %q, want enabled/starting", snapshot.DesiredState, snapshot.RuntimeState)
	}
	if got := snapshot.Commands[0].Usage; got != "/订阅b站推送 UID或昵称" {
		t.Fatalf("command usage = %q, want UID或昵称", got)
	}
	if got := snapshot.Help.Summary; got != "新帮助摘要" {
		t.Fatalf("help summary = %q, want 新帮助摘要", got)
	}
	if len(snapshot.Permissions) != 2 {
		t.Fatalf("permissions = %#v, want http.request and message.send", snapshot.Permissions)
	}
}

func TestReloadSyncsPluginRenderTemplates(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	renderRoot := filepath.Join(t.TempDir(), "render")
	pluginID := "weather-card"
	templateID := "card"
	writePluginRenderTemplate(t, repoRoot, pluginID, templateID)
	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", pluginID)
	runner := &captureRenderRunner{}
	renderer := newRenderServiceForRepo(t, repoRoot, renderRoot, runner)
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          pluginID,
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		PackageRootPath:   pluginRoot,
		RenderTemplates:   []plugins.RenderTemplate{{Path: "templates/" + templateID}},
	}})
	if err := renderer.SyncPluginTemplateDeclarations(context.Background(), testRenderTemplateDeclarations(catalog.List())); err != nil {
		t.Fatalf("initial sync plugin render templates: %v", err)
	}

	request := render.Request{
		Template: "plugin.weather-card.card",
		Output:   "png",
		Data: map[string]any{
			"title": "天气卡片",
		},
	}
	first, err := renderer.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("initial render: %v", err)
	}
	if first.FromCache {
		t.Fatalf("initial render unexpectedly used cache")
	}
	if html := runner.lastHTML(); !strings.Contains(html, "<body>天气卡片</body>") {
		t.Fatalf("initial render html = %s, want original template", html)
	}

	templatePath := filepath.Join(pluginRoot, "templates", templateID, "template.html")
	if err := os.WriteFile(templatePath, []byte("<html><body>fresh {{ .title }}</body></html>"), 0o644); err != nil {
		t.Fatalf("write updated plugin template: %v", err)
	}

	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(t,
		catalog,
		nil,
		pluginruntime.NewRegistry(slog.Default(), pluginruntime.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	app.services.pluginLifecycle.syncRenderTemplates = func(ctx context.Context) error {
		return renderer.SyncPluginTemplateDeclarations(ctx, testRenderTemplateDeclarations(catalog.List()))
	}

	if _, err := app.services.pluginLifecycle.Reload(context.Background(), pluginID); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	second, err := renderer.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("render after reload: %v", err)
	}
	if second.FromCache {
		t.Fatalf("expected reload-synced template render to miss previous cache")
	}
	if second.ArtifactID == first.ArtifactID || second.CacheKey == first.CacheKey {
		t.Fatalf("render after reload reused old artifact/cache: first=%s/%s second=%s/%s", first.ArtifactID, first.CacheKey, second.ArtifactID, second.CacheKey)
	}
	if html := runner.lastHTML(); !strings.Contains(html, "<body>fresh 天气卡片</body>") {
		t.Fatalf("render after reload html = %s, want updated template", html)
	}
}

func testRenderTemplateDeclarations(snapshots []plugins.Snapshot) []render.PluginTemplateDeclaration {
	var declarations []render.PluginTemplateDeclaration
	for _, snapshot := range snapshots {
		for _, declared := range snapshot.RenderTemplates {
			declarations = append(declarations, render.PluginTemplateDeclaration{
				PluginID:          snapshot.PluginID,
				Path:              declared.Path,
				PackageRootPath:   snapshot.PackageRootPath,
				Valid:             snapshot.Valid,
				RegistrationState: snapshot.RegistrationState,
			})
		}
	}
	return declarations
}

func TestReloadReturnsTemplateSyncErrorBeforeStartingRuntime(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(t,
		catalog,
		nil,
		pluginruntime.NewRegistry(slog.Default(), pluginruntime.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	syncErr := errors.New("sync plugin templates")
	app.services.pluginLifecycle.syncRenderTemplates = func(context.Context) error {
		return syncErr
	}

	_, err := app.services.pluginLifecycle.Reload(context.Background(), "weather-card")
	if !errors.Is(err, syncErr) {
		t.Fatalf("Reload error = %v, want sync error", err)
	}
	snapshot, ok := catalog.Get("weather-card")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.RuntimeState != "running" {
		t.Fatalf("runtime_state = %q, want running", snapshot.RuntimeState)
	}
}

func TestPluginRuntimeStartInputsIncludeSuperAdmins(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginRoot := writeInstallSourcePlugin(t, filepath.Join(repoRoot, "plugins", "weather-card"), "weather-card")
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		ManifestPath:      "plugins/weather-card/info.json",
		PackageRootPath:   pluginRoot,
	}})
	app := newTestAppState(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"10001", "10002", "10001", " "},
		},
	}, slog.Default())
	app.state.repoRoot = repoRoot
	app.setTestLifecycle(t,
		catalog,
		nil,
		pluginruntime.NewRegistry(slog.Default(), pluginruntime.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)

	_, payload, err := app.services.pluginLifecycle.buildStartInputs(context.Background(), "weather-card")
	if err != nil {
		t.Fatalf("buildStartInputs: %v", err)
	}
	if !reflect.DeepEqual(payload.SuperAdmins, []string{"10001", "10002"}) {
		t.Fatalf("super_admins = %#v, want canonical values", payload.SuperAdmins)
	}
	app.state.Config.Scheduler.Timezone = "America/Los_Angeles"
	_, pending, err := app.services.pluginLifecycle.buildStartInputs(context.Background(), "weather-card")
	if err != nil {
		t.Fatal(err)
	}
	if pending.Timezone != "Asia/Shanghai" {
		t.Fatalf("pending setting changed plugin timezone before restart: %q", pending.Timezone)
	}
	app.setTestLifecycle(t, catalog, nil, pluginruntime.NewRegistry(slog.Default(), pluginruntime.Options{}), dispatch.New(slog.Default(), nil, nil, 16), nil, nil, newPluginWebhookRegistry())
	_, restarted, err := app.services.pluginLifecycle.buildStartInputs(context.Background(), "weather-card")
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Timezone != "America/Los_Angeles" {
		t.Fatalf("restarted plugin timezone = %q", restarted.Timezone)
	}
}

func TestRefreshPluginManifestReadsUpdatedManifestFile(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginDir := filepath.Join(repoRoot, "plugins", "installed", "subscription_hub")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifestPath := filepath.Join(pluginDir, "info.json")
	writeInstallSourcePlugin(t, pluginDir, "raylea.subscription-hub")
	writeLifecyclePluginManifest(t, manifestPath, "/订阅b站推送 UID")

	validator := compilePluginValidatorForLifecycleTest(t)
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: validator,
		Roots: []plugincatalog.ScanRoot{{
			Label: "plugins/installed",
			Path:  filepath.Join(repoRoot, "plugins", "installed"),
		}},
		RepoRoot: repoRoot,
	})
	if err != nil {
		t.Fatalf("Discover initial: %v", err)
	}
	catalog := plugincatalog.New(snapshots)
	if updated, err := catalog.SetRuntimeState("raylea.subscription-hub", "running"); err != nil || updated.RuntimeState != "running" {
		t.Fatalf("SetRuntimeState: snapshot=%+v err=%v", updated, err)
	}

	writeLifecyclePluginManifest(t, manifestPath, "/订阅b站推送 UID或昵称")
	refreshed, err := RefreshPluginManifest(context.Background(), catalog, nil, "raylea.subscription-hub", func() ([]plugins.Snapshot, error) {
		snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
			Validator: validator,
			Roots: []plugincatalog.ScanRoot{{
				Label: "plugins/installed",
				Path:  filepath.Join(repoRoot, "plugins", "installed"),
			}},
			RepoRoot: repoRoot,
		})
		return snapshots, err
	})
	if err != nil {
		t.Fatalf("RefreshPluginManifest: %v", err)
	}
	if refreshed.RuntimeState != "running" {
		t.Fatalf("refreshed runtime_state = %q, want running", refreshed.RuntimeState)
	}
	if got := refreshed.Commands[0].Usage; got != "/订阅b站推送 UID或昵称" {
		t.Fatalf("command usage = %q, want UID或昵称", got)
	}
	if _, ok := refreshed.Permissions["http.request"]; !ok {
		t.Fatalf("permissions = %#v, want http.request", refreshed.Permissions)
	}
}

func compilePluginValidatorForLifecycleTest(t *testing.T) *config.Validator {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	validator, err := config.Compile(filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin manifest schema: %v", err)
	}
	return validator
}

func writeLifecyclePluginManifest(t *testing.T, path, usage string) {
	t.Helper()

	content := `{
  "id": "raylea.subscription-hub",
  "name": "Subscription Hub",
  "version": "0.4.0",
  "manifest_version": "3",
  "license": "MIT",
  "min_core_version": "0.4.0",
  "metadata": {"description": "Subscription hub", "author": "raylea"},
	"permissions": {"http.request": true},
  "commands": [
    {
      "id": "subscribe-bilibili",
      "name": "订阅 Bilibili 推送",
      "description": "订阅 Bilibili 推送",
      "usage": ` + quoteLifecycleJSON(usage) + `,
      "permission": "super_admin",
      "trigger": {"type": "exact", "names": ["订阅b站推送"]}
    }
  ],
  "help": {
    "title": "订阅中心",
    "summary": "管理订阅来源"
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	refreshInstallArtifact(t, filepath.Dir(path))
}

func quoteLifecycleJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
