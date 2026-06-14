package service

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
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func TestReloadDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})

	_, err := controller.Reload(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected Reload to fail for expired required grant")
	}
	if _, ok := err.(*plugins.PermissionPendingError); !ok {
		t.Fatalf("err = %T, want *plugins.PermissionPendingError", err)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestReloadReturnsPermissionPendingWhenGrantScopeChanged(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ScopeJSON:  `{"http_hosts":["api.example"]}`,
	}})

	_, err := controller.Reload(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected Reload to fail when grant scope changed")
	}
	pending, ok := err.(*plugins.PermissionPendingError)
	if !ok {
		t.Fatalf("err = %T, want *plugins.PermissionPendingError", err)
	}
	if !pending.ScopeChanged {
		t.Fatalf("ScopeChanged = %v, want true", pending.ScopeChanged)
	}
	if len(pending.MissingCapabilities) != 0 {
		t.Fatalf("MissingCapabilities = %#v, want empty", pending.MissingCapabilities)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestReloadRefreshesManifestCommandsAndScopes(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:            "raylea.subscription-hub",
		Name:                "Subscription Hub",
		Valid:               true,
		SourceRoot:          "plugins/builtin",
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"http.request"},
		ScopeHTTPHosts:      []string{"old.example"},
		Commands: []plugins.Command{{
			Name:          "订阅b站推送",
			Usage:         "/订阅b站推送 UID",
			CommandSource: plugins.CommandSourceManifest,
		}},
		Help: &plugins.Help{
			Title: "订阅中心",
			Groups: []plugins.HelpGroup{{
				Title: "订阅操作",
				Items: []plugins.HelpItem{{
					Title: "订阅 Bilibili 推送",
					Usage: "/订阅b站推送 UID",
				}},
			}},
		},
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(
		catalog,
		nil,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	app.services.pluginLifecycle.refreshManifest = func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return RefreshPluginManifest(ctx, catalog, nil, pluginID, func() ([]plugins.Snapshot, error) {
			return []plugins.Snapshot{{
				PluginID:            "raylea.subscription-hub",
				Name:                "Subscription Hub",
				Valid:               true,
				SourceRoot:          "plugins/builtin",
				RegistrationState:   "installed",
				DesiredState:        "enabled",
				RuntimeState:        "stopped",
				RequiredPermissions: []string{"http.request"},
				ScopeHTTPHosts:      []string{"api.bilibili.com", "api.live.bilibili.com"},
				ManifestCommands: []plugins.Command{{
					Name:          "订阅b站推送",
					Usage:         "/订阅b站推送 UID或昵称",
					CommandSource: plugins.CommandSourceManifest,
				}},
				Commands: []plugins.Command{{
					Name:          "订阅b站推送",
					Usage:         "/订阅b站推送 UID或昵称",
					CommandSource: plugins.CommandSourceManifest,
				}},
				Help: &plugins.Help{
					Title: "订阅中心",
					Groups: []plugins.HelpGroup{{
						Title: "订阅操作",
						Items: []plugins.HelpItem{{
							Title: "订阅 Bilibili 推送",
							Usage: "/订阅b站推送 UID或昵称",
						}},
					}},
				},
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
	if got := snapshot.Help.Groups[0].Items[0].Usage; got != "/订阅b站推送 UID或昵称" {
		t.Fatalf("help usage = %q, want UID或昵称", got)
	}
	hosts := app.services.pluginLifecycle.grants.GrantedHTTPHosts(context.Background(), "raylea.subscription-hub")
	if !reflect.DeepEqual(hosts, []string{"api.bilibili.com", "api.live.bilibili.com"}) {
		t.Fatalf("http hosts = %#v, want Bilibili hosts", hosts)
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
	if err := SyncCatalogRenderTemplates(context.Background(), renderer, catalog); err != nil {
		t.Fatalf("initial sync plugin render templates: %v", err)
	}

	request := renderservice.Request{
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
	app.setTestLifecycle(
		catalog,
		nil,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	app.services.pluginLifecycle.syncRenderTemplates = func(ctx context.Context) error {
		return SyncCatalogRenderTemplates(ctx, renderer, catalog)
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
	app.setTestLifecycle(
		catalog,
		nil,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
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
	writeManagedRuntimeFixtures(t, repoRoot)
	createPluginEntry(t, repoRoot, "plugins/weather-card", "main.py")
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Runtime:           "python",
		Entry:             "main.py",
		ManifestPath:      "plugins/weather-card/info.json",
	}})
	app := newTestAppState(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"10001", "10002", "10001", " "},
		},
	}, slog.Default())
	app.state.repoRoot = repoRoot
	app.setTestLifecycle(
		catalog,
		nil,
		nil,
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)

	_, payload, err := app.services.pluginLifecycle.buildStartInputsWithCapabilities("weather-card", "", []string{"event.subscribe"})
	if err != nil {
		t.Fatalf("buildStartInputsWithCapabilities: %v", err)
	}
	if !reflect.DeepEqual(payload.SuperAdmins, []string{"10001", "10002"}) {
		t.Fatalf("super_admins = %#v, want canonical values", payload.SuperAdmins)
	}
}

func TestRefreshPluginManifestReadsUpdatedManifestFile(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginDir := filepath.Join(repoRoot, "plugins", "builtin", "subscription_hub")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifestPath := filepath.Join(pluginDir, "info.json")
	writeLifecyclePluginManifest(t, manifestPath, "/订阅b站推送 UID", "old.example")

	validator := compilePluginValidatorForLifecycleTest(t)
	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: validator,
		Roots: []plugindiscovery.ScanRoot{{
			Label: "plugins/builtin",
			Path:  filepath.Join(repoRoot, "plugins", "builtin"),
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

	writeLifecyclePluginManifest(t, manifestPath, "/订阅b站推送 UID或昵称", "api.bilibili.com")
	refreshed, err := RefreshPluginManifest(context.Background(), catalog, nil, "raylea.subscription-hub", func() ([]plugins.Snapshot, error) {
		snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
			Validator: validator,
			Roots: []plugindiscovery.ScanRoot{{
				Label: "plugins/builtin",
				Path:  filepath.Join(repoRoot, "plugins", "builtin"),
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
	if got := refreshed.ScopeHTTPHosts; !reflect.DeepEqual(got, []string{"api.bilibili.com"}) {
		t.Fatalf("http hosts = %#v, want api.bilibili.com", got)
	}
}

func TestReconcileRuntimeDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})

	controller.reconcileRuntime(context.Background(), "10001")

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestStartRuntimeDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})
	manager := runtimemanager.New(slog.Default(), runtimemanager.Options{})

	err := controller.startRuntime(context.Background(), "weather", "10001", manager)
	if err == nil {
		t.Fatal("expected startRuntime to fail for expired grant")
	}
	if _, ok := err.(*plugins.PermissionPendingError); !ok {
		t.Fatalf("err = %T, want *plugins.PermissionPendingError", err)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func compilePluginValidatorForLifecycleTest(t *testing.T) *schema.Validator {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	validator, err := schema.Compile(filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin manifest schema: %v", err)
	}
	return validator
}

func writeLifecyclePluginManifest(t *testing.T, path, usage, host string) {
	t.Helper()

	content := `{
  "id": "raylea.subscription-hub",
  "name": "Subscription Hub",
  "version": "0.1.0",
  "manifest_version": "1",
  "plugin_protocol_version": "1",
  "type": "managed_runtime",
  "runtime": "python",
  "entry": "main.py",
  "license": "MIT",
  "description": "Subscription hub",
  "author": "raylea",
  "permissions": {
    "required": ["http.request"],
    "optional": [],
    "scopes": {
      "http_hosts": [` + quoteLifecycleJSON(host) + `]
    }
  },
  "commands": [
    {
      "name": "订阅b站推送",
      "description": "订阅 Bilibili 推送",
      "usage": ` + quoteLifecycleJSON(usage) + `,
      "permission": "super_admin"
    }
  ],
  "help": {
    "title": "订阅中心",
    "groups": [
      {
        "title": "订阅操作",
        "items": [
          {
            "title": "订阅 Bilibili 推送",
            "description": "指定 Bilibili 用户",
            "usage": ` + quoteLifecycleJSON(usage) + `,
            "command": "订阅b站推送",
            "permission": "super_admin"
          }
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func quoteLifecycleJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func newLifecycleControllerForGrantTests(t *testing.T, grants []plugins.PluginGrant) (*Controller, *plugincatalog.Catalog) {
	t.Helper()

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"http.request"},
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(
		catalog,
		nil,
		&stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"weather": grants,
			},
		},
		newRuntimeRegistry(slog.Default(), runtimemanager.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	return app.services.pluginLifecycle, catalog
}

type stubLifecycleGrantRepository struct {
	grants map[string][]plugins.PluginGrant
}

func (r *stubLifecycleGrantRepository) LoadGrants(_ context.Context, pluginID string) ([]plugins.PluginGrant, error) {
	now := time.Now().UTC()
	var active []plugins.PluginGrant
	for _, grant := range r.grants[pluginID] {
		if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
			continue
		}
		active = append(active, grant)
	}
	return active, nil
}

func (r *stubLifecycleGrantRepository) LoadAllGrants(_ context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	for pluginID := range r.grants {
		items, _ := r.LoadGrants(context.Background(), pluginID)
		for _, grant := range items {
			result[pluginID] = append(result[pluginID], grant.Capability)
		}
	}
	return result, nil
}

func (r *stubLifecycleGrantRepository) SaveGrant(context.Context, plugins.PluginGrant) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteGrant(context.Context, string, string) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteAllGrants(context.Context, string) error {
	return nil
}

func timePtr(value time.Time) *time.Time {
	return &value
}
