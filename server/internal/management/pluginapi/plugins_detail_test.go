package pluginapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestDetailHandler_ReturnsPermissionSummaries(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"weather": {{
				PluginID:   "weather",
				Capability: "logger.write",
				GrantedAt:  time.Now().UTC(),
			}},
		},
	}
	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:            "weather",
		Name:                "Weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		OptionalPermissions: []string{"logger.write"},
		RequiredPermissions: []string{"http.request"},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, repo, func() []string {
		return []string{"http.request"}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.Permissions) != 2 {
		t.Fatalf("len(permissions) = %d, want 2", len(resp.Plugin.Permissions))
	}
	if resp.Plugin.Permissions[0].Capability != "http.request" || resp.Plugin.Permissions[0].Source != string(plugins.PermissionSourceConfigAuto) {
		t.Fatalf("unexpected first permission: %#v", resp.Plugin.Permissions[0])
	}
	if resp.Plugin.Permissions[1].Capability != "logger.write" || resp.Plugin.Permissions[1].Source != string(plugins.PermissionSourcePersisted) {
		t.Fatalf("unexpected second permission: %#v", resp.Plugin.Permissions[1])
	}
}

func TestDetailHandler_ReturnsBuiltinAutoPermissions(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:            "raylea.echo",
		Name:                "Echo",
		Valid:               true,
		SourceRoot:          "plugins/builtin",
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"message.send"},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/raylea.echo", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.Permissions) != 1 {
		t.Fatalf("len(permissions) = %d, want 1", len(resp.Plugin.Permissions))
	}
	for _, permission := range resp.Plugin.Permissions {
		if permission.Source != string(plugins.PermissionSourceBuiltinAuto) {
			t.Fatalf("permission source = %q, want %q", permission.Source, plugins.PermissionSourceBuiltinAuto)
		}
		if permission.Status != string(plugins.PermissionStatusGranted) {
			t.Fatalf("permission status = %q, want %q", permission.Status, plugins.PermissionStatusGranted)
		}
	}
}

func TestDetailHandlerReturnsHelpProjection(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Help: &plugins.Help{
			Title:   "Weather",
			Summary: "天气菜单",
			Groups: []plugins.HelpGroup{{
				Title: "查询",
				Items: []plugins.HelpItem{{
					Title:       "城市天气",
					Description: "查询城市天气",
					Usage:       "/weather 上海",
					Command:     "weather",
					Permission:  "everyone",
				}},
			}},
		},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.Help.Title != "Weather" {
		t.Fatalf("unexpected help projection: %#v", resp.Plugin.Help)
	}
	if len(resp.Plugin.Help.Groups) != 1 || resp.Plugin.Help.Groups[0].Title != "查询" {
		t.Fatalf("unexpected help groups: %#v", resp.Plugin.Help.Groups)
	}
	if got := resp.Plugin.Help.Groups[0].Items[0]; got.Command != "weather" || got.Title != "城市天气" {
		t.Fatalf("unexpected help item: %#v", got)
	}
}

func TestDetailHandler_ReturnsManagementUI(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "example-config-panel",
		Name:              "Example Config Panel",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
		ManagementUI: &plugins.ManagementUI{
			Pages: []plugins.ManagementUIPage{
				{ID: "config", Label: "配置页面", Entry: "web/index.html"},
				{ID: "secrets", Label: "密钥设置", Entry: "web/secrets.html"},
			},
		},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/example-config-panel", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.ManagementUI == nil {
		t.Fatal("expected management_ui in detail response")
	}
	if len(resp.Plugin.ManagementUI.Pages) != 2 {
		t.Fatalf("management_ui.pages length = %d, want 2", len(resp.Plugin.ManagementUI.Pages))
	}
	if got := resp.Plugin.ManagementUI.Pages[1]; got.ID != "secrets" || got.Label != "密钥设置" || got.Entry != "web/secrets.html" {
		t.Fatalf("unexpected management_ui page: %#v", got)
	}
}

func TestDetailHandler_ReturnsRenderTemplates(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Name:              "Weather Card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
		RenderTemplates:   []plugins.RenderTemplate{{Path: "templates/card"}},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather-card", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.RenderTemplates) != 1 || resp.Plugin.RenderTemplates[0].Path != "templates/card" {
		t.Fatalf("render_templates = %#v, want templates/card", resp.Plugin.RenderTemplates)
	}
}
