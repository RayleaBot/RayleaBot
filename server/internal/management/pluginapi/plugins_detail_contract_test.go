package pluginapi

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestGetPluginReturnsValidSnapshot(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "hello-python",
			Name:              "Hello Python",
			Role:              "example",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "discovered",
			SourceRoot:        "examples/plugins",
			Commands: []plugins.Command{
				{
					Name:        "hello",
					Aliases:     []string{"hi"},
					Description: "Say hello",
					Usage:       "hello",
					Permission:  "member",
				},
			},
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins/hello-python", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	want := map[string]any{
		"plugin": map[string]any{
			"id":                 "hello-python",
			"name":               "Hello Python",
			"role":               "example",
			"registration_state": "installed",
			"desired_state":      "disabled",
			"runtime_state":      "stopped",
			"display_state":      "discovered",
			"source": map[string]any{
				"root":     "examples/plugins",
				"verified": true,
			},
			"trust": map[string]any{
				"level": "third_party",
				"label": "示例",
			},
			"commands": []any{
				map[string]any{
					"name":           "hello",
					"aliases":        []any{"hi"},
					"description":    "Say hello",
					"usage":          "hello",
					"permission":     "member",
					"command_source": "manifest",
				},
			},
			"help": map[string]any{
				"groups": []any{},
			},
			"command_conflicts": []any{},
			"permissions":       []any{},
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("unexpected body: got %#v want %#v", body, want)
	}
}

func TestGetPluginReturnsRichMetadataDetail(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:             "weather",
			Name:                 "Weather",
			Role:                 "user",
			Version:              "1.4.2",
			Runtime:              "python",
			Type:                 "managed_runtime",
			Entry:                "plugin.py",
			Description:          "提供当前城市天气与未来天气查询。",
			Author:               "raylea",
			License:              "MIT",
			SDKMinVersion:        "1.2.0",
			RuntimeVersion:       ">=3.12",
			MinCoreVersion:       "0.2.0",
			DataSchemaVersion:    "weather-v2",
			Concurrency:          3,
			Platforms:            []string{"windows-x64", "linux-x64"},
			DefaultConfig:        map[string]any{"unit": "metric", "forecast_days": 3},
			DeclaredCapabilities: []string{"http.request", "logger.write", "render.image"},
			PythonDependencies:   []string{"httpx==0.28.1"},
			ScopeHTTPHosts:       []string{"api.weather.example"},
			ScopeStorageRoots:    []string{"plugin_data"},
			Icon:                 "assets/weather.svg",
			Repo:                 "https://github.com/RayleaBot/plugins-weather",
			Homepage:             "https://plugins.rayleabot.local/weather",
			Keywords:             []string{"weather", "forecast", "climate"},
			Screenshots: []plugins.Screenshot{{
				Path: "assets/overview.svg",
				Alt:  "天气总览卡片",
			}},
			SystemDependencies: []string{"OneBot11 connection"},
			Valid:              true,
			RegistrationState:  "installed",
			DesiredState:       "enabled",
			RuntimeState:       "running",
			DisplayState:       "running",
			SourceRoot:         "plugins/installed",
			PackageSourceType:  "local_zip",
			PackageSourceRef:   "C:/plugins/weather.zip",
			Commands: []plugins.Command{{
				Name:        "weather",
				Aliases:     []string{"tq", "天气"},
				Description: "查询天气",
				Usage:       "weather <城市>",
				Permission:  "member",
			}},
			RequiredPermissions: []string{"http.request"},
			OptionalPermissions: []string{"logger.write", "render.image"},
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins/weather", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	plugin := body["plugin"].(map[string]any)
	if plugin["version"] != "1.4.2" {
		t.Fatalf("unexpected version: %#v", plugin["version"])
	}
	if plugin["author"] != "raylea" || plugin["license"] != "MIT" {
		t.Fatalf("unexpected author/license: %#v", plugin)
	}
	if plugin["sdk_min_version"] != "1.2.0" || plugin["runtime_version"] != ">=3.12" {
		t.Fatalf("unexpected sdk/runtime version fields: %#v", plugin)
	}
	if plugin["concurrency"] != float64(3) {
		t.Fatalf("unexpected concurrency: %#v", plugin["concurrency"])
	}
	if !reflect.DeepEqual(plugin["keywords"], []any{"weather", "forecast", "climate"}) {
		t.Fatalf("unexpected keywords: %#v", plugin["keywords"])
	}
	if !reflect.DeepEqual(plugin["system_dependencies"], []any{"OneBot11 connection"}) {
		t.Fatalf("unexpected system_dependencies: %#v", plugin["system_dependencies"])
	}
	dependencies := plugin["dependencies"].(map[string]any)
	if !reflect.DeepEqual(dependencies["python"], []any{"httpx==0.28.1"}) {
		t.Fatalf("unexpected dependencies: %#v", dependencies)
	}
	scopes := plugin["scopes"].(map[string]any)
	if !reflect.DeepEqual(scopes["http_hosts"], []any{"api.weather.example"}) {
		t.Fatalf("unexpected scope http_hosts: %#v", scopes["http_hosts"])
	}
	if !reflect.DeepEqual(scopes["storage_roots"], []any{"plugin_data"}) {
		t.Fatalf("unexpected scope storage_roots: %#v", scopes["storage_roots"])
	}
	screenshots := plugin["screenshots"].([]any)
	if len(screenshots) != 1 {
		t.Fatalf("unexpected screenshots: %#v", screenshots)
	}
	screenshot := screenshots[0].(map[string]any)
	if screenshot["path"] != "assets/overview.svg" || screenshot["alt"] != "天气总览卡片" {
		t.Fatalf("unexpected screenshot: %#v", screenshot)
	}
	defaultConfig := plugin["default_config"].(map[string]any)
	if defaultConfig["unit"] != "metric" || defaultConfig["forecast_days"] != float64(3) {
		t.Fatalf("unexpected default_config: %#v", defaultConfig)
	}
	if !reflect.DeepEqual(plugin["declared_capabilities"], []any{"http.request", "logger.write", "render.image"}) {
		t.Fatalf("unexpected declared_capabilities: %#v", plugin["declared_capabilities"])
	}
}

func TestGetPluginReturns404WhenMissing(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugincatalog.New(nil))

	request := httptest.NewRequest("GET", "/api/plugins/missing-plugin", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["resource_type"] != "plugin" {
		t.Fatalf("unexpected resource_type: %#v", details["resource_type"])
	}
}

func TestInvalidPluginAppearsInListButDetailReturns409(t *testing.T) {
	t.Parallel()

	snapshot := plugins.Snapshot{
		PluginID:          "unsupported-binary-tool",
		Valid:             false,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "invalid_manifest",
		ManifestPath:      "plugins/installed/unsupported-binary-tool/info.json",
		ValidationSummary: "runtime must be one of python or nodejs",
		Commands: []plugins.Command{
			{Name: "unsupported"},
		},
	}
	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot}))

	listRequest := httptest.NewRequest("GET", "/api/plugins", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != 200 {
		t.Fatalf("unexpected list status: got %d want 200", listRecorder.Code)
	}
	listBody := decodeBody(t, listRecorder.Body.Bytes())
	items := listBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected list count: got %d want 1", len(items))
	}
	item := items[0].(map[string]any)
	if commands := item["commands"].([]any); len(commands) != 0 {
		t.Fatalf("invalid plugin commands = %#v, want []", commands)
	}

	detailRequest := httptest.NewRequest("GET", "/api/plugins/unsupported-binary-tool", nil)
	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != 409 {
		t.Fatalf("unexpected detail status: got %d want 409", detailRecorder.Code)
	}

	detailBody := decodeBody(t, detailRecorder.Body.Bytes())
	errorBody := detailBody["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["kind"] != "invalid_manifest" {
		t.Fatalf("unexpected error kind: %#v", details["kind"])
	}
}

func TestConflictPluginDetailReturns409(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Valid:             false,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "conflict",
			ValidationSummary: "duplicate plugin_id discovered across multiple directories",
			ConflictPaths: []string{
				"examples/plugins/weather/info.json",
				"plugins/installed/weather/info.json",
			},
			SourceRoots: []string{"examples/plugins", "plugins/installed"},
			Commands: []plugins.Command{
				{Name: "weather"},
			},
		},
	}))

	listRequest := httptest.NewRequest("GET", "/api/plugins", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != 200 {
		t.Fatalf("unexpected list status: got %d want 200", listRecorder.Code)
	}

	listBody := decodeBody(t, listRecorder.Body.Bytes())
	items := listBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected list count: got %d want 1", len(items))
	}
	item := items[0].(map[string]any)
	if commands := item["commands"].([]any); len(commands) != 0 {
		t.Fatalf("conflict plugin commands = %#v, want []", commands)
	}

	request := httptest.NewRequest("GET", "/api/plugins/weather", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 409 {
		t.Fatalf("unexpected status: got %d want 409", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["kind"] != "plugin_id_conflict" {
		t.Fatalf("unexpected conflict kind: %#v", details["kind"])
	}
	if len(details["manifest_paths"].([]any)) != 2 {
		t.Fatalf("unexpected manifest_paths length: %#v", details["manifest_paths"])
	}
}
