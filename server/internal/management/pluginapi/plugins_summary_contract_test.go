package pluginapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestListPluginsReturnsContractShape(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "raylea.echo",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			DisplayState:      "running",
			Name:              "Echo",
			Description:       "Built-in echo command",
			SourceRoot:        "plugins/builtin",
			Commands: []plugins.Command{
				{Name: "echo"},
			},
			Help: &plugins.Help{
				Title:   "Echo",
				Summary: "Built-in echo command",
				Groups: []plugins.HelpGroup{{
					Title: "基础指令",
					Items: []plugins.HelpItem{{
						Title:       "复读内容",
						Description: "复读收到的内容",
						Usage:       "/echo <内容>",
						Command:     "echo",
						Permission:  "everyone",
					}},
				}},
			},
		},
		{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			DisplayState:      "running",
			Name:              "Weather",
			Role:              "user",
			SourceRoot:        "plugins/installed",
			PackageSourceType: "local_zip",
			PackageSourceRef:  "C:/plugins/weather.zip",
			Commands: []plugins.Command{
				{
					Name:        "weather",
					Aliases:     []string{"天气"},
					Description: "查询天气",
					Usage:       "weather <城市>",
					Permission:  "member",
				},
			},
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
		},
		{
			PluginID:          "weather-admin",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			DisplayState:      "running",
			Name:              "Weather Admin",
			Role:              "dev",
			SourceRoot:        "plugins/dev",
			Commands: []plugins.Command{
				{Name: "weather"},
			},
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", body["items"])
	}
	if len(items) != 3 {
		t.Fatalf("unexpected item count: got %d want 3", len(items))
	}

	byID := make(map[string]map[string]any, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected item object, got %#v", item)
		}
		allowed := map[string]bool{
			"id":                true,
			"name":              true,
			"version":           true,
			"description":       true,
			"author":            true,
			"role":              true,
			"state":             true,
			"state_diagnosis":   true,
			"source":            true,
			"trust":             true,
			"commands":          true,
			"help":              true,
			"command_conflicts": true,
		}
		for key := range itemMap {
			if !allowed[key] {
				t.Fatalf("unexpected public field %q in list response", key)
			}
		}
		byID[itemMap["id"].(string)] = itemMap
	}

	builtin := byID["raylea.echo"]
	if builtin["state"] != "running" {
		t.Fatalf("raylea.echo state = %v, want running", builtin["state"])
	}
	if builtin["role"] != "builtin" {
		t.Fatalf("raylea.echo role = %v, want builtin", builtin["role"])
	}
	if conflicts := builtin["command_conflicts"].([]any); len(conflicts) != 0 {
		t.Fatalf("raylea.echo command_conflicts = %#v, want []", conflicts)
	}
	assertCommandList(t, builtin["commands"], []map[string]any{
		{
			"name":           "echo",
			"command_source": "manifest",
		},
	})
	assertPluginHelp(t, builtin["help"], "Echo", "基础指令", "复读内容")

	weather := byID["weather"]
	if weather["name"] != "Weather" {
		t.Fatalf("weather name = %v, want Weather", weather["name"])
	}
	if weather["role"] != "user" {
		t.Fatalf("weather role = %v, want user", weather["role"])
	}
	source := weather["source"].(map[string]any)
	if source["root"] != "plugins/installed" {
		t.Fatalf("weather source.root = %v, want plugins/installed", source["root"])
	}
	if source["package_source_type"] != "local_zip" {
		t.Fatalf("weather package_source_type = %v, want local_zip", source["package_source_type"])
	}
	if source["package_source_ref"] != "C:/plugins/weather.zip" {
		t.Fatalf("weather package_source_ref = %v, want C:/plugins/weather.zip", source["package_source_ref"])
	}
	if source["verified"] != false {
		t.Fatalf("weather verified = %v, want false", source["verified"])
	}
	trust := weather["trust"].(map[string]any)
	if trust["level"] != "unverified" {
		t.Fatalf("weather trust.level = %v, want unverified", trust["level"])
	}
	if trust["label"] != "未验证来源" {
		t.Fatalf("weather trust.label = %v, want 未验证来源", trust["label"])
	}
	if conflicts := weather["command_conflicts"].([]any); len(conflicts) != 1 || conflicts[0] != "weather" {
		t.Fatalf("weather command_conflicts = %#v, want [weather]", conflicts)
	}
	assertCommandList(t, weather["commands"], []map[string]any{
		{
			"name":           "weather",
			"aliases":        []any{"天气"},
			"description":    "查询天气",
			"usage":          "weather <城市>",
			"permission":     "member",
			"command_source": "manifest",
		},
	})
	assertPluginHelp(t, weather["help"], "Weather", "查询", "城市天气")

	devPlugin := byID["weather-admin"]
	if devPlugin["role"] != "dev" {
		t.Fatalf("weather-admin role = %v, want dev", devPlugin["role"])
	}
	devTrust := devPlugin["trust"].(map[string]any)
	if devTrust["level"] != "development" {
		t.Fatalf("weather-admin trust.level = %v, want development", devTrust["level"])
	}
	if devTrust["label"] != "开发中" {
		t.Fatalf("weather-admin trust.label = %v, want 开发中", devTrust["label"])
	}
	assertCommandList(t, devPlugin["commands"], []map[string]any{
		{
			"name":           "weather",
			"command_source": "manifest",
		},
	})
}
