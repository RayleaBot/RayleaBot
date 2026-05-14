package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestListPluginsReturnsContractShape(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
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
			"id":                 true,
			"name":               true,
			"role":               true,
			"registration_state": true,
			"desired_state":      true,
			"runtime_state":      true,
			"display_state":      true,
			"source":             true,
			"trust":              true,
			"commands":           true,
			"help":               true,
			"command_conflicts":  true,
		}
		for key := range itemMap {
			if !allowed[key] {
				t.Fatalf("unexpected public field %q in list response", key)
			}
		}
		byID[itemMap["id"].(string)] = itemMap
	}

	builtin := byID["raylea.echo"]
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

func TestGetPluginReturnsValidSnapshot(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
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

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
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

	router := pluginRouter(t, plugins.NewCatalog(nil))

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
		PluginID:          "legacy-binary-tool",
		Valid:             false,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "invalid_manifest",
		ManifestPath:      "plugins/installed/legacy-binary-tool/info.json",
		ValidationSummary: "runtime must be one of python or nodejs",
		Commands: []plugins.Command{
			{Name: "legacy"},
		},
	}
	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{snapshot}))

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

	detailRequest := httptest.NewRequest("GET", "/api/plugins/legacy-binary-tool", nil)
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

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
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

func pluginRouter(t *testing.T, catalog *plugins.Catalog) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	plugins.RegisterRoutes(router, catalog, nil, nil, nil, nil, nil, nil, nil)
	return router
}

func pluginRouterWithController(t *testing.T, catalog *plugins.Catalog, controller plugins.DesiredStateController, uninstaller plugins.UninstallCoordinator) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	plugins.RegisterRoutes(router, catalog, nil, nil, nil, controller, uninstaller, nil, nil)
	return router
}

type stubReloadController struct {
	reloadResult plugins.Snapshot
	reloadErr    error
}

func (s *stubReloadController) Enable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}
func (s *stubReloadController) Disable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}
func (s *stubReloadController) Reload(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.reloadResult, s.reloadErr
}
func (s *stubReloadController) RecoverFromDeadLetter(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}

type stubUninstallCoordinator struct {
	taskID string
	err    error
}

func (s *stubUninstallCoordinator) Accept(_ context.Context, _ string) (string, error) {
	return s.taskID, s.err
}

func assertCommandList(t *testing.T, got any, want []map[string]any) {
	t.Helper()

	items, ok := got.([]any)
	if !ok {
		t.Fatalf("expected commands array, got %#v", got)
	}
	if len(items) != len(want) {
		t.Fatalf("unexpected command count: got %d want %d", len(items), len(want))
	}
	for index, expected := range want {
		command, ok := items[index].(map[string]any)
		if !ok {
			t.Fatalf("expected command object, got %#v", items[index])
		}
		if !reflect.DeepEqual(command, expected) {
			t.Fatalf("unexpected command at index %d: got %#v want %#v", index, command, expected)
		}
	}
}

func TestReloadPluginReturnsUpdatedSnapshot(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"},
	})
	controller := &stubReloadController{
		reloadResult: plugins.Snapshot{
			PluginID: "weather", RegistrationState: "installed", DesiredState: "enabled",
			RuntimeState: "starting", DisplayState: "enabling",
		},
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/weather/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	plugin := body["plugin"].(map[string]any)
	if plugin["desired_state"] != "enabled" {
		t.Fatalf("desired_state should remain enabled, got %v", plugin["desired_state"])
	}
	if plugin["runtime_state"] != "starting" {
		t.Fatalf("runtime_state should be starting, got %v", plugin["runtime_state"])
	}
}

func TestReloadPluginRejectsDisabledPlugin(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"},
	})
	controller := &stubReloadController{
		reloadErr: plugins.ErrStateConflict,
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/weather/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 409 {
		t.Fatalf("unexpected status: got %d want 409", recorder.Code)
	}
}

func TestReloadPluginRejectsNotFound(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog(nil)
	controller := &stubReloadController{
		reloadErr: plugins.ErrPluginNotFound,
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/nonexistent/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}
}

func TestUninstallPluginReturnsTaskAccepted(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"},
	})
	uninstaller := &stubUninstallCoordinator{taskID: "task_plugin_uninstall_0001"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/weather", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 202 {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	if body["task_id"] != "task_plugin_uninstall_0001" {
		t.Fatalf("unexpected task_id: %v", body["task_id"])
	}
}

func TestUninstallPluginRejectsNotFound(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog(nil)
	uninstaller := &stubUninstallCoordinator{taskID: "should-not-reach"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/nonexistent", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error code: %v", errorBody["code"])
	}
}

func TestUninstallBuiltinPluginRejected(t *testing.T) {
	t.Parallel()

	catalog := plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:          "raylea.echo",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "stopped",
			SourceRoot:        "plugins/builtin",
		},
	})
	uninstaller := &stubUninstallCoordinator{taskID: "should-not-run"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/raylea.echo", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 409 {
		t.Fatalf("unexpected status: got %d want 409", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %v", errorBody["code"])
	}
}

func assertPluginHelp(t *testing.T, got any, wantTitle string, wantGroup string, wantItemTitle string) {
	t.Helper()

	help, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected help object, got %#v", got)
	}
	if help["title"] != wantTitle {
		t.Fatalf("help.title = %#v, want %q", help["title"], wantTitle)
	}
	groups, ok := help["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("unexpected help groups: %#v", help["groups"])
	}
	group := groups[0].(map[string]any)
	if group["title"] != wantGroup {
		t.Fatalf("help group title = %#v, want %q", group["title"], wantGroup)
	}
	items := group["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected help group items: %#v", group["items"])
	}
	item := items[0].(map[string]any)
	if item["title"] != wantItemTitle {
		t.Fatalf("help item title = %#v, want %q", item["title"], wantItemTitle)
	}
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	return body
}
