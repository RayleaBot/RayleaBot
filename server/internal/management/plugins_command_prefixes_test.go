package management

import (
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/go-chi/chi/v5"
)

func TestPluginReadsUseCurrentCommandPrefixes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		declared  *plugins.ManifestCommandPrefixes
		settings  map[string]any
		initial   []string
		updated   []string
		dedicated []string
	}{
		{
			name:    "仅通用前缀",
			initial: []string{"/", "#"}, updated: []string{"?", "#"}, dedicated: []string{},
		},
		{
			name:     "专属前缀替换且拒绝通用前缀",
			declared: &plugins.ManifestCommandPrefixes{Dedicated: []string{"*", "星铁"}, SettingsKey: "prefixes"},
			settings: map[string]any{"prefixes": []string{"sr", "星穹铁道"}},
			initial:  []string{"*", "星铁"}, updated: []string{"sr", "星穹铁道"}, dedicated: []string{"sr", "星穹铁道"},
		},
		{
			name:     "专属前缀在前并对通用前缀去重",
			declared: &plugins.ManifestCommandPrefixes{Dedicated: []string{"*", "星铁", "#"}, SettingsKey: "prefixes", AcceptGlobal: true},
			settings: map[string]any{"prefixes": []string{"sr", "#"}},
			initial:  []string{"*", "星铁", "#", "/"}, updated: []string{"sr", "#", "?"}, dedicated: []string{"sr", "#"},
		},
		{
			name:     "拒绝通用前缀时空设置保留声明",
			declared: &plugins.ManifestCommandPrefixes{Dedicated: []string{"%", "绝区零"}, SettingsKey: "prefixes"},
			settings: map[string]any{"prefixes": []string{}},
			initial:  []string{"%", "绝区零"}, updated: []string{"%", "绝区零"}, dedicated: []string{"%", "绝区零"},
		},
		{
			name:     "接受通用前缀时可清空专属前缀",
			declared: &plugins.ManifestCommandPrefixes{Dedicated: []string{"*"}, SettingsKey: "prefixes", AcceptGlobal: true},
			settings: map[string]any{"prefixes": []string{}},
			initial:  []string{"*", "/", "#"}, updated: []string{"?", "#"}, dedicated: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := plugins.Snapshot{
				PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped",
				ManifestCommandPrefixes: tc.declared,
			}
			snapshot.CommandPrefixes = plugincatalog.ProjectCommandPrefixes(snapshot, nil)
			catalog := plugincatalog.New([]plugins.Snapshot{snapshot})
			var current atomic.Pointer[config.Config]
			current.Store(&config.Config{Command: &config.CommandConfig{Prefixes: []string{"/", "#"}}})
			router := chi.NewRouter()
			registerPluginReadRoutes(router, catalog, func() config.Config { return *current.Load() })
			initialDedicated := append([]string{}, snapshot.CommandPrefixes.Dedicated...)
			for _, route := range []string{"/api/plugins", "/api/plugins/fixture"} {
				assertPluginResponsePrefixes(t, router, "GET", route, tc.initial, initialDedicated)
			}

			if _, ok := catalog.RefreshCommands("fixture", tc.settings); !ok {
				t.Fatal("未刷新插件设置投影")
			}
			current.Store(&config.Config{Command: &config.CommandConfig{Prefixes: []string{"?", "#"}}})
			for _, route := range []string{"/api/plugins", "/api/plugins/fixture"} {
				assertPluginResponsePrefixes(t, router, "GET", route, tc.updated, tc.dedicated)
			}
		})
	}
}

func TestPluginLifecycleResponsesUseCommandPrefixes(t *testing.T) {
	t.Parallel()
	for _, ignoreGlobal := range []bool{false, true} {
		snapshot := plugins.Snapshot{
			PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "starting",
			CommandPrefixes: plugins.CommandPrefixes{Dedicated: []string{"%", "绝区零"}, IgnoreGlobal: ignoreGlobal},
		}
		catalog := plugincatalog.New([]plugins.Snapshot{{PluginID: "fixture"}})
		controller := &stubDesiredStateController{
			enableResult: snapshot, disableResult: snapshot, reloadResult: snapshot, recoverResult: snapshot,
		}
		var current atomic.Pointer[config.Config]
		current.Store(&config.Config{})
		currentConfig := func() config.Config { return *current.Load() }
		router := chi.NewRouter()
		registerPluginLifecycleRoutes(router, catalog, controller, nil, currentConfig)
		registerPluginDeadLetterRoutes(router, catalog, controller, currentConfig)
		for _, global := range []string{"!", "?"} {
			current.Store(&config.Config{Command: &config.CommandConfig{Prefixes: []string{global}}})
			want := []string{"%", "绝区零"}
			if !ignoreGlobal {
				want = append(want, global)
			}
			for _, action := range []string{"enable", "disable", "reload", "recover"} {
				assertPluginResponsePrefixes(t, router, "POST", "/api/plugins/fixture/"+action, want, []string{"%", "绝区零"})
			}
		}
	}
}

func TestPluginRoutesRequireCurrentConfig(t *testing.T) {
	t.Parallel()
	routes, err := NewPluginRoutes(PluginRouteDeps{
		Catalog: plugincatalog.New(nil), Installer: testInstallCoordinator{}, Uninstaller: &stubUninstallCoordinator{},
		Lifecycle: &pluginservice.Controller{},
	})
	if err == nil || routes != nil {
		t.Fatal("缺少当前配置依赖时应拒绝装配插件路由")
	}
}

func assertPluginResponsePrefixes(t *testing.T, router *chi.Mux, method, route string, all, dedicated []string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, route, nil))
	if recorder.Code != 200 {
		t.Fatalf("%s %s：状态码 %d，响应 %s", method, route, recorder.Code, recorder.Body.String())
	}
	body := decodeBody(t, recorder.Body.Bytes())
	var plugin map[string]any
	if route == "/api/plugins" {
		items := body["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("插件条目 = %#v", items)
		}
		plugin = items[0].(map[string]any)
	} else {
		plugin = body["plugin"].(map[string]any)
	}
	for field, want := range map[string][]string{"command_prefixes": all, "dedicated_command_prefixes": dedicated} {
		raw, ok := plugin[field].([]any)
		if !ok {
			t.Fatalf("%s %s 未返回数组：%#v", route, field, plugin[field])
		}
		got := make([]string, 0, len(raw))
		for _, value := range raw {
			prefix, ok := value.(string)
			if !ok {
				t.Fatalf("%s 包含非字符串前缀：%#v", field, value)
			}
			got = append(got, prefix)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s %s = %v，预期 %v", route, field, got, want)
		}
	}
}
