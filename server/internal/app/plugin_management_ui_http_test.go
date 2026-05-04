package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type pluginManagementUIErrorEnvelope struct {
	Error struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func openPluginSettingsRepo(t *testing.T) pluginconfig.Repository {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	repo, err := pluginconfig.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("pluginconfig.NewSQLiteRepository: %v", err)
	}
	return repo
}

func TestHandlePluginManagementUIStaticServesScopedAssets(t *testing.T) {
	t.Parallel()

	pluginDir := filepath.Join(t.TempDir(), "example-config-panel")
	webDir := filepath.Join(pluginDir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><title>Config Panel</title>"), 0o644); err != nil {
		t.Fatalf("os.WriteFile index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "app.js"), []byte("console.log('config panel')"), 0o644); err != nil {
		t.Fatalf("os.WriteFile app.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.py"), []byte("print('plugin')"), 0o644); err != nil {
		t.Fatalf("os.WriteFile main.py: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugins.NewCatalog([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			PackageRootPath:   pluginDir,
			ManagementUI: &plugins.ManagementUI{
				Entry: "web/index.html",
			},
		}}),
	})
	router := chi.NewRouter()
	router.Get("/plugin-ui/{plugin_id}/*", handlers.handlePluginManagementUIStatic())

	entryRequest := httptest.NewRequest(http.MethodGet, "/plugin-ui/example-config-panel/web/index.html", nil)
	entryRecorder := httptest.NewRecorder()
	router.ServeHTTP(entryRecorder, entryRequest)

	if entryRecorder.Code != http.StatusOK {
		t.Fatalf("entry status = %d, want 200; body=%s", entryRecorder.Code, entryRecorder.Body.String())
	}
	if body := entryRecorder.Body.String(); body != "<!doctype html><title>Config Panel</title>" {
		t.Fatalf("unexpected entry body: %q", body)
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/plugin-ui/example-config-panel/web/app.js", nil)
	assetRecorder := httptest.NewRecorder()
	router.ServeHTTP(assetRecorder, assetRequest)

	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want 200; body=%s", assetRecorder.Code, assetRecorder.Body.String())
	}
	if body := assetRecorder.Body.String(); body != "console.log('config panel')" {
		t.Fatalf("unexpected asset body: %q", body)
	}
}

func TestHandlePluginManagementUIStaticRejectsParentEscape(t *testing.T) {
	t.Parallel()

	pluginDir := filepath.Join(t.TempDir(), "example-config-panel")
	webDir := filepath.Join(pluginDir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.py"), []byte("print('plugin')"), 0o644); err != nil {
		t.Fatalf("os.WriteFile main.py: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugins.NewCatalog([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			PackageRootPath:   pluginDir,
			ManagementUI: &plugins.ManagementUI{
				Entry: "web/index.html",
			},
		}}),
	})
	router := chi.NewRouter()
	router.Get("/plugin-ui/{plugin_id}/*", handlers.handlePluginManagementUIStatic())

	request := httptest.NewRequest(http.MethodGet, "/plugin-ui/example-config-panel/web/../main.py", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestHandlePluginSettingsGetMergesDefaultsAndPersistedValues(t *testing.T) {
	t.Parallel()

	repo := openPluginSettingsRepo(t)
	if _, err := repo.Write(context.Background(), "example-config-panel", map[string]any{
		"default_city": "上海",
		"timezone":     "Asia/Shanghai",
		"unit":         "fahrenheit",
	}); err != nil {
		t.Fatalf("repo.Write: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugins.NewCatalog([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DefaultConfig: map[string]any{
				"default_city": "北京",
				"unit":         "celsius",
			},
		}}),
		pluginConfig: repo,
	})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}/settings", handlers.handlePluginSettingsGet())

	request := httptest.NewRequest(http.MethodGet, "/api/plugins/example-config-panel/settings", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var response pluginSettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.PluginID != "example-config-panel" {
		t.Fatalf("plugin_id = %q, want example-config-panel", response.PluginID)
	}
	if response.Values["default_city"] != "上海" {
		t.Fatalf("default_city = %#v, want 上海", response.Values["default_city"])
	}
	if response.Values["unit"] != "fahrenheit" {
		t.Fatalf("unit = %#v, want fahrenheit", response.Values["unit"])
	}
	if response.Values["timezone"] != "Asia/Shanghai" {
		t.Fatalf("timezone = %#v, want Asia/Shanghai", response.Values["timezone"])
	}
}

func TestHandlePluginSettingsPutDispatchesConfigChanged(t *testing.T) {
	t.Parallel()

	repo := openPluginSettingsRepo(t)
	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	application := newTestAppState(config.Config{}, nil)
	catalog := plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "example-config-panel",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DefaultConfig: map[string]any{
			"default_city":     "北京",
			"unit":             "celsius",
			"trigger_commands": []any{"默认指令"},
		},
		DynamicCommands: []plugins.DynamicCommandDecl{{
			ID:          "dynamic",
			SettingsKey: "trigger_commands",
			Description: "动态指令",
		}},
	}})
	application.plugins = catalog
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		repo,
		nil,
		nil,
		nil,
		dispatcher,
		nil,
		nil,
		nil,
		nil,
	)
	fakeRuntime := &capturingRuntime{events: make(chan runtime.Event, 1)}
	dispatcher.Register("example-config-panel", fakeRuntime, []string{"config.changed"}, nil, 1)

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins:            catalog,
		pluginConfig:       repo,
		notifyConfigChange: application.dispatchPluginConfigChanged,
		refreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			applicationRefreshPluginCommands(catalog, dispatcher, pluginID, settings)
		},
	})
	router := chi.NewRouter()
	router.Put("/api/plugins/{plugin_id}/settings", handlers.handlePluginSettingsPut())

	body := bytes.NewReader([]byte(`{"values":{"default_city":"上海","unit":"fahrenheit","trigger_commands":["今日签"]}}`))
	request := httptest.NewRequest(http.MethodPut, "/api/plugins/example-config-panel/settings", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var response pluginSettingsUpdateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.ChangedKeys) != 3 || response.ChangedKeys[0] != "default_city" || response.ChangedKeys[1] != "trigger_commands" || response.ChangedKeys[2] != "unit" {
		t.Fatalf("unexpected changed_keys: %#v", response.ChangedKeys)
	}
	if response.Values["default_city"] != "上海" || response.Values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected values: %#v", response.Values)
	}
	snapshot, ok := catalog.Get("example-config-panel")
	if !ok {
		t.Fatal("expected plugin snapshot")
	}
	if len(snapshot.Commands) != 1 || snapshot.Commands[0].Name != "今日签" || snapshot.Commands[0].CommandSource != plugins.CommandSourceDynamic {
		t.Fatalf("unexpected refreshed commands: %#v", snapshot.Commands)
	}

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "config.changed" {
			t.Fatalf("event_type = %q, want config.changed", event.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected config.changed event")
	}
}

func TestHandlePluginSettingsRejectsInvalidPluginSnapshots(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		path         string
		snapshot     plugins.Snapshot
		wantStatus   int
		wantCode     string
		wantKind     string
		wantResource string
	}{
		{
			name: "invalid manifest",
			path: "/api/plugins/example-config-panel/settings",
			snapshot: plugins.Snapshot{
				PluginID:          "example-config-panel",
				Valid:             false,
				DisplayState:      "invalid_manifest",
				ManifestPath:      "plugins/example-config-panel/info.json",
				ValidationSummary: "manifest invalid",
			},
			wantStatus: http.StatusConflict,
			wantCode:   "platform.invalid_request",
			wantKind:   "invalid_manifest",
		},
		{
			name: "removed",
			path: "/api/plugins/example-config-panel/settings",
			snapshot: plugins.Snapshot{
				PluginID:          "example-config-panel",
				Valid:             true,
				RegistrationState: "removed",
			},
			wantStatus: http.StatusConflict,
			wantCode:   "platform.invalid_request",
			wantKind:   "plugin_not_installed",
		},
		{
			name:         "missing",
			path:         "/api/plugins/missing/settings",
			wantStatus:   http.StatusNotFound,
			wantCode:     "platform.resource_missing",
			wantResource: "plugin",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var entries []plugins.Snapshot
			if tc.snapshot.PluginID != "" {
				entries = []plugins.Snapshot{tc.snapshot}
			}

			handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
				plugins: plugins.NewCatalog(entries),
			})
			router := chi.NewRouter()
			router.Get("/api/plugins/{plugin_id}/settings", handlers.handlePluginSettingsGet())

			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}

			var env pluginManagementUIErrorEnvelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if env.Error.Code != tc.wantCode {
				t.Fatalf("error.code = %q, want %q", env.Error.Code, tc.wantCode)
			}
			if tc.wantKind != "" && env.Error.Details["kind"] != tc.wantKind {
				t.Fatalf("details.kind = %#v, want %q", env.Error.Details["kind"], tc.wantKind)
			}
			if tc.wantResource != "" && env.Error.Details["resource_type"] != tc.wantResource {
				t.Fatalf("details.resource_type = %#v, want %q", env.Error.Details["resource_type"], tc.wantResource)
			}
		})
	}
}
