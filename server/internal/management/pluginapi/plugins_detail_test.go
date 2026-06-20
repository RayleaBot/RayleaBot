package pluginapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestDetailHandler_ReturnsDeclaredCapabilitiesAndParameters(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:             "weather",
		Name:                 "Weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "enabled",
		RuntimeState:         "running",
		DeclaredCapabilities: []string{"http.request", "logger.write", "storage.file"},
		ScopeHTTPHosts:       []string{"api.weather.example"},
		ScopeStorageRoots:    []string{"plugin_data"},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))

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
	if len(resp.Plugin.DeclaredCapabilities) != 3 {
		t.Fatalf("declared_capabilities = %#v, want 3 items", resp.Plugin.DeclaredCapabilities)
	}
	if resp.Plugin.CapabilityParameters == nil {
		t.Fatal("capability_parameters is nil")
	}
	if len(resp.Plugin.CapabilityParameters.HTTPHosts) != 1 || resp.Plugin.CapabilityParameters.HTTPHosts[0] != "api.weather.example" {
		t.Fatalf("http_hosts = %#v", resp.Plugin.CapabilityParameters.HTTPHosts)
	}
	if len(resp.Plugin.CapabilityParameters.StorageRoots) != 1 || resp.Plugin.CapabilityParameters.StorageRoots[0] != "plugin_data" {
		t.Fatalf("storage_roots = %#v", resp.Plugin.CapabilityParameters.StorageRoots)
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
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))

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
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))

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
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))

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
