package management

import (
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestGetPluginReturnsV3CommandProjection(t *testing.T) {
	t.Parallel()
	snapshot := plugins.Snapshot{
		PluginID: "hello-go", Name: "Hello Go", Valid: true,
		RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped",
		SourceRoot: "plugins/installed", PackageSourceType: "development", PackageSourceRef: "C:/workspace/hello-go",
		Commands: []plugins.Command{{
			ID: "hello", Name: "hello", DisplayName: "Hello", Aliases: []string{"hi"},
			TriggerType: "exact", TriggerNames: []string{"hello", "hi"},
			Description: "Say hello", Usage: "/hello", Permission: "everyone",
		}},
	}
	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins/hello-go", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := decodeBody(t, recorder.Body.Bytes())
	plugin := body["plugin"].(map[string]any)
	commands := plugin["commands"].([]any)
	command := commands[0].(map[string]any)
	if command["id"] != "hello" || command["name"] != "Hello" {
		t.Fatalf("command = %#v", command)
	}
	trigger := command["trigger"].(map[string]any)
	if trigger["type"] != "exact" || len(trigger["names"].([]any)) != 2 {
		t.Fatalf("trigger = %#v", trigger)
	}
	if _, exists := command["command_source"]; exists {
		t.Fatalf("legacy command_source leaked: %#v", command)
	}
}

func TestGetPluginReturnsRichV3Metadata(t *testing.T) {
	t.Parallel()
	snapshot := plugins.Snapshot{
		PluginID: "weather", Name: "Weather", Version: "1.4.2", Description: "天气查询",
		Author: "raylea", License: "MIT", MinCoreVersion: "0.4.0", Concurrency: 3,
		Events: []string{"message.group"},
		Permissions: map[string]bool{
			"http.request": true, "secret.write": true,
		},
		Icon: "assets/weather.svg", Repo: "https://github.com/RayleaBot/plugins-weather",
		Homepage: "https://plugins.rayleabot.local/weather", Keywords: []string{"weather", "forecast"},
		Screenshots: []plugins.Screenshot{{Path: "assets/overview.svg", Alt: "天气总览"}},
		Valid:       true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		SourceRoot: "plugins/installed", PackageSourceType: "local_zip", PackageSourceRef: "C:/plugins/weather.zip",
	}
	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins/weather", nil))
	body := decodeBody(t, recorder.Body.Bytes())
	plugin := body["plugin"].(map[string]any)
	if plugin["version"] != "1.4.2" || plugin["min_core_version"] != "0.4.0" || plugin["concurrency"] != float64(3) {
		t.Fatalf("metadata = %#v", plugin)
	}
	permissions := plugin["permissions"].(map[string]any)
	if permissions["http.request"] != true {
		t.Fatalf("permissions = %#v", permissions)
	}
	if _, exists := plugin["default_config"]; exists {
		t.Fatalf("default_config leaked: %#v", plugin)
	}
}

func TestGetPluginReturns404WhenMissing(t *testing.T) {
	t.Parallel()
	router := pluginRouter(t, plugincatalog.New(nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins/missing-plugin", nil))
	if recorder.Code != 404 {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := decodeBody(t, recorder.Body.Bytes())
	if body["error"].(map[string]any)["code"] != "platform.resource_not_found" {
		t.Fatalf("body = %#v", body)
	}
}

func TestInvalidAndConflictedPluginsExposeNoCommands(t *testing.T) {
	t.Parallel()
	for _, snapshot := range []plugins.Snapshot{
		{PluginID: "invalid", RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped", DisplayState: "invalid_manifest", ValidationSummary: "unsupported contract"},
		{PluginID: "conflict", RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped", DisplayState: "conflict", ValidationSummary: "duplicate plugin id", ConflictPaths: []string{"a/info.json", "b/info.json"}},
	} {
		snapshot := snapshot
		t.Run(snapshot.PluginID, func(t *testing.T) {
			router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot}))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins/"+snapshot.PluginID, nil))
			plugin := decodeBody(t, recorder.Body.Bytes())["plugin"].(map[string]any)
			if plugin["state"] != "invalid" || len(plugin["commands"].([]any)) != 0 {
				t.Fatalf("plugin = %#v", plugin)
			}
		})
	}
}
