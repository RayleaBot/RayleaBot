package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestDetailHandlerReturnsPermissions(t *testing.T) {
	t.Parallel()
	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID: "weather", Name: "Weather", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Permissions: map[string]plugins.PermissionGrant{
			"http.request":            {},
			"thirdparty.account.read": {Platforms: []string{"bilibili", "weibo"}},
		},
	}})
	response := requestPluginDetail(t, catalog, "weather")
	if response.Plugin.Permissions["http.request"] != true {
		t.Fatalf("http.request permission = %#v", response.Plugin.Permissions["http.request"])
	}
	grant, ok := response.Plugin.Permissions["thirdparty.account.read"].(map[string]any)
	if !ok {
		t.Fatalf("thirdparty permission = %#v", response.Plugin.Permissions["thirdparty.account.read"])
	}
	platforms, _ := grant["platforms"].([]any)
	if len(platforms) != 2 || platforms[0] != "bilibili" || platforms[1] != "weibo" {
		t.Fatalf("permission platforms = %#v", platforms)
	}
}

func TestDetailHandlerReturnsGeneratedHelpMetadata(t *testing.T) {
	t.Parallel()
	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID: "weather", Name: "Weather", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Help: &plugins.Help{Title: "Weather", Summary: "天气命令"},
		Commands: []plugins.Command{{
			ID: "weather", Name: "weather", DisplayName: "天气", TriggerType: "exact", TriggerNames: []string{"weather"},
			Description: "查询天气", Usage: "/weather 上海", Permission: "everyone",
		}},
		CommandGroups: []plugins.CommandGroup{{ID: "query", Title: "查询", Commands: []string{"weather"}}},
	}})
	response := requestPluginDetail(t, catalog, "weather")
	if response.Plugin.Help.Title != "Weather" || response.Plugin.Help.Summary != "天气命令" {
		t.Fatalf("help = %#v", response.Plugin.Help)
	}
	if len(response.Plugin.CommandGroups) != 1 || response.Plugin.CommandGroups[0].Commands[0] != "weather" {
		t.Fatalf("command_groups = %#v", response.Plugin.CommandGroups)
	}
}

func TestDetailHandlerReturnsSingleManagementUIEntry(t *testing.T) {
	t.Parallel()
	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID: "example-config-panel", Name: "Example Config Panel", Valid: true,
		RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped",
		ManagementUI: &plugins.ManagementUI{
			Entry: "ui/index.html",
			Pages: []plugins.ManagementUIPage{{ID: "config", Label: "配置"}, {ID: "secrets", Label: "密钥"}},
		},
	}})
	response := requestPluginDetail(t, catalog, "example-config-panel")
	if response.Plugin.ManagementUI == nil || response.Plugin.ManagementUI.Entry != "ui/index.html" {
		t.Fatalf("management_ui = %#v", response.Plugin.ManagementUI)
	}
	if len(response.Plugin.ManagementUI.Pages) != 2 || response.Plugin.ManagementUI.Pages[1].ID != "secrets" {
		t.Fatalf("management_ui.pages = %#v", response.Plugin.ManagementUI.Pages)
	}
}

func requestPluginDetail(t *testing.T, catalog plugins.CatalogView, pluginID string) DetailResponse {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/"+pluginID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response DetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response
}
