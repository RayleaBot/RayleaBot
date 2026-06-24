package services

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

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
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

func openPluginSecretStore(t *testing.T) secrets.Store {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("secrets.NewSQLiteStore: %v", err)
	}
	return secretStore
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
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			PackageRootPath:   pluginDir,
			ManagementUI: &plugins.ManagementUI{
				Pages: []plugins.ManagementUIPage{
					{ID: "config", Label: "配置页面", Entry: "web/index.html"},
				},
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
	assertPluginUIStaticNoStoreHeaders(t, entryRecorder.Header())

	assetRequest := httptest.NewRequest(http.MethodGet, "/plugin-ui/example-config-panel/web/app.js", nil)
	assetRecorder := httptest.NewRecorder()
	router.ServeHTTP(assetRecorder, assetRequest)

	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want 200; body=%s", assetRecorder.Code, assetRecorder.Body.String())
	}
	if body := assetRecorder.Body.String(); body != "console.log('config panel')" {
		t.Fatalf("unexpected asset body: %q", body)
	}
	assertPluginUIStaticNoStoreHeaders(t, assetRecorder.Header())
}

func assertPluginUIStaticNoStoreHeaders(t *testing.T, header http.Header) {
	t.Helper()

	if got := header.Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("Cache-Control = %q, want no-store, max-age=0", got)
	}
	if got := header.Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", got)
	}
	if got := header.Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q, want 0", got)
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
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			PackageRootPath:   pluginDir,
			ManagementUI: &plugins.ManagementUI{
				Pages: []plugins.ManagementUIPage{
					{ID: "config", Label: "配置页面", Entry: "web/index.html"},
				},
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
		plugins: plugincatalog.New([]plugins.Snapshot{{
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

	var response pluginui.PluginSettingsResponse
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
	catalog := plugincatalog.New([]plugins.Snapshot{{
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
	application.pluginStack.Plugins = catalog
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{}},
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
	fakeRuntime := &capturingRuntime{events: make(chan runtimeprotocol.Event, 1)}
	dispatcher.Register("example-config-panel", fakeRuntime, []string{"config.changed"}, nil, 1)

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins:            catalog,
		pluginConfig:       repo,
		notifyConfigChange: application.dispatchPluginConfigChanged,
		refreshCommands:    localaction.RefreshCommands(catalog, dispatcher),
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

	var response pluginui.PluginSettingsUpdateResponse
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

func TestHandlePluginSecretsGetAndPutAreScopedToPlugin(t *testing.T) {
	t.Parallel()

	secretStore := openPluginSecretStore(t)
	sealedPrimary, err := secrets.SealString(context.Background(), secretStore, "SESSDATA=fixture")
	if err != nil {
		t.Fatalf("secrets.SealString primary: %v", err)
	}
	if err := secretStore.Set(context.Background(), "plugin:example-config-panel:secret:bili_token_primary", sealedPrimary); err != nil {
		t.Fatalf("secretStore.Set: %v", err)
	}
	sealedOther, err := secrets.SealString(context.Background(), secretStore, "SESSDATA=other")
	if err != nil {
		t.Fatalf("secrets.SealString other: %v", err)
	}
	if err := secretStore.Set(context.Background(), "plugin:other-plugin:secret:bili_token_primary", sealedOther); err != nil {
		t.Fatalf("secretStore.Set other: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
		}}),
		secrets: secretStore,
	})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}/secrets", handlers.handlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", handlers.handlePluginSecretsPut())

	getRequest := httptest.NewRequest(http.MethodGet, "/api/plugins/example-config-panel/secrets", nil)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	var getResponse pluginui.PluginSecretsResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResponse.Values["bili_token_primary"] != "SESSDATA=fixture" {
		t.Fatalf("unexpected values: %#v", getResponse.Values)
	}
	if _, exists := getResponse.Values["other-plugin"]; exists {
		t.Fatalf("unexpected cross-plugin secret: %#v", getResponse.Values)
	}

	body := bytes.NewReader([]byte(`{"values":{"bili_token_backup":"SESSDATA=backup"},"deleted_keys":["bili_token_primary"]}`))
	putRequest := httptest.NewRequest(http.MethodPut, "/api/plugins/example-config-panel/secrets", body)
	putRequest.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	router.ServeHTTP(putRecorder, putRequest)

	if putRecorder.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", putRecorder.Code, putRecorder.Body.String())
	}

	var putResponse pluginui.PluginSecretsUpdateResponse
	if err := json.Unmarshal(putRecorder.Body.Bytes(), &putResponse); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if len(putResponse.ChangedKeys) != 2 || putResponse.ChangedKeys[0] != "bili_token_backup" || putResponse.ChangedKeys[1] != "bili_token_primary" {
		t.Fatalf("unexpected changed_keys: %#v", putResponse.ChangedKeys)
	}
	if putResponse.Values["bili_token_backup"] != "SESSDATA=backup" {
		t.Fatalf("unexpected updated values: %#v", putResponse.Values)
	}
	if _, exists := putResponse.Values["bili_token_primary"]; exists {
		t.Fatalf("deleted secret still returned: %#v", putResponse.Values)
	}
	storedBackup, err := secretStore.Get(context.Background(), "plugin:example-config-panel:secret:bili_token_backup")
	if err != nil {
		t.Fatalf("stored backup missing: %v", err)
	}
	if string(storedBackup) == "SESSDATA=backup" {
		t.Fatal("plugin secret was stored as plaintext")
	}
	openedBackup, err := secrets.OpenString(context.Background(), secretStore, storedBackup)
	if err != nil || openedBackup != "SESSDATA=backup" {
		t.Fatalf("backup decrypt = %q err=%v", openedBackup, err)
	}
	if other, err := secretStore.Get(context.Background(), "plugin:other-plugin:secret:bili_token_primary"); err != nil {
		t.Fatalf("cross-plugin secret missing: %v", err)
	} else if opened, err := secrets.OpenString(context.Background(), secretStore, other); err != nil || opened != "SESSDATA=other" {
		t.Fatalf("cross-plugin secret changed: value=%q err=%v", opened, err)
	}
}

func TestHandlePluginSecretsPutRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	secretStore := openPluginSecretStore(t)
	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "example-config-panel",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
		}}),
		secrets: secretStore,
	})
	router := chi.NewRouter()
	router.Put("/api/plugins/{plugin_id}/secrets", handlers.handlePluginSecretsPut())

	body := bytes.NewReader([]byte(`{"values":{"Bad Key":"SESSDATA=fixture"}}`))
	request := httptest.NewRequest(http.MethodPut, "/api/plugins/example-config-panel/secrets", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
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
				plugins: plugincatalog.New(entries),
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
