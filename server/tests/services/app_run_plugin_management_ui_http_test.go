package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
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

func openPluginSettingsRepo(t *testing.T) pluginstore.ConfigRepository {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatalf("pluginstore.NewConfigSQLiteRepository: %v", err)
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
	uiDir := filepath.Join(pluginDir, "ui")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<!doctype html><title>Config Panel</title>"), 0o644); err != nil {
		t.Fatalf("os.WriteFile index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "app.js"), []byte("console.log('config panel')"), 0o644); err != nil {
		t.Fatalf("os.WriteFile app.js: %v", err)
	}
	binDir := filepath.Join(pluginDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "example-config-panel"), []byte("go plugin fixture"), 0o755); err != nil {
		t.Fatalf("os.WriteFile backend: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:            "example-config-panel",
			Valid:               true,
			RegistrationState:   "installed",
			DesiredState:        "disabled",
			RuntimeState:        "stopped",
			PackageRootPath:     pluginDir,
			ArtifactVersion:     "1",
			ArtifactUIAvailable: true,
			ManagementUI: &plugins.ManagementUI{
				Pages: []plugins.ManagementUIPage{
					{ID: "config", Label: "配置页面", Entry: "ui/index.html"},
				},
			},
		}}),
	})
	options := managementapi.PluginUIOriginOptions{ServerPort: 8080, AdminOrigins: []string{"http://127.0.0.1:8080"}}
	origin, err := managementapi.PluginUIOrigin("example-config-panel", options)
	if err != nil {
		t.Fatalf("PluginUIOrigin: %v", err)
	}
	handler := handlers.IsolatedOriginHandler(http.NotFoundHandler(), options)

	entryRequest := httptest.NewRequest(http.MethodGet, origin+"/", nil)
	entryRecorder := httptest.NewRecorder()
	handler.ServeHTTP(entryRecorder, entryRequest)

	if entryRecorder.Code != http.StatusOK {
		t.Fatalf("entry status = %d, want 200; body=%s", entryRecorder.Code, entryRecorder.Body.String())
	}
	if body := entryRecorder.Body.String(); body != "<!doctype html><title>Config Panel</title>" {
		t.Fatalf("unexpected entry body: %q", body)
	}
	assertPluginUIStaticNoStoreHeaders(t, entryRecorder.Header())

	assetRequest := httptest.NewRequest(http.MethodGet, origin+"/app.js", nil)
	assetRecorder := httptest.NewRecorder()
	handler.ServeHTTP(assetRecorder, assetRequest)

	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want 200; body=%s", assetRecorder.Code, assetRecorder.Body.String())
	}
	if body := assetRecorder.Body.String(); body != "console.log('config panel')" {
		t.Fatalf("unexpected asset body: %q", body)
	}
	assertPluginUIStaticNoStoreHeaders(t, assetRecorder.Header())

	apiRequest := httptest.NewRequest(http.MethodGet, origin+"/api/config", nil)
	apiRequest.Header.Set("Origin", "http://127.0.0.1:8080")
	apiRecorder := httptest.NewRecorder()
	handler.ServeHTTP(apiRecorder, apiRequest)
	if apiRecorder.Code != http.StatusNotFound {
		t.Fatalf("plugin-origin API status = %d, want 404", apiRecorder.Code)
	}
	if got := apiRecorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("plugin-origin API exposed CORS: %q", got)
	}
	if got := apiRecorder.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("plugin-origin API set cookies: %#v", got)
	}
}

func TestPluginUIOriginRejectsAdminOrigin(t *testing.T) {
	t.Parallel()
	pluginID := "example-config-panel"
	digest := sha256.Sum256([]byte(pluginID))
	pluginHost := fmt.Sprintf("p-%x", digest[:8])
	template := "https://{plugin_host}.plugins.example.com"
	adminOrigin := "https://" + pluginHost + ".plugins.example.com:443"
	if _, err := managementapi.PluginUIOrigin(pluginID, managementapi.PluginUIOriginOptions{
		OriginTemplate: template,
		AdminOrigins:   []string{adminOrigin},
	}); err == nil {
		t.Fatal("PluginUIOrigin accepted the admin origin")
	}
}

func assertPluginUIStaticNoStoreHeaders(t *testing.T, header http.Header) {
	t.Helper()

	if got := header.Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("Cache-Control = %q, want no-store, max-age=0", got)
	}
	if got := header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := header.Get("Content-Security-Policy"); !strings.Contains(got, "connect-src 'none'") || !strings.Contains(got, "frame-ancestors http://127.0.0.1:8080") {
		t.Fatalf("unexpected Content-Security-Policy: %q", got)
	}
}

func TestHandlePluginManagementUIStaticRejectsParentEscape(t *testing.T) {
	t.Parallel()

	pluginDir := filepath.Join(t.TempDir(), "example-config-panel")
	uiDir := filepath.Join(pluginDir, "ui")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	binDir := filepath.Join(pluginDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "example-config-panel"), []byte("go plugin fixture"), 0o755); err != nil {
		t.Fatalf("os.WriteFile backend: %v", err)
	}

	handlers := newPluginManagementUIHTTPHandlers(pluginManagementUIHTTPDeps{
		plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:            "example-config-panel",
			Valid:               true,
			RegistrationState:   "installed",
			DesiredState:        "disabled",
			RuntimeState:        "stopped",
			PackageRootPath:     pluginDir,
			ArtifactVersion:     "1",
			ArtifactUIAvailable: true,
			ManagementUI: &plugins.ManagementUI{
				Pages: []plugins.ManagementUIPage{
					{ID: "config", Label: "配置页面", Entry: "ui/index.html"},
				},
			},
		}}),
	})
	options := managementapi.PluginUIOriginOptions{ServerPort: 8080}
	origin, err := managementapi.PluginUIOrigin("example-config-panel", options)
	if err != nil {
		t.Fatalf("PluginUIOrigin: %v", err)
	}
	handler := handlers.IsolatedOriginHandler(http.NotFoundHandler(), options)

	request := httptest.NewRequest(http.MethodGet, origin+"/../bin/example-config-panel", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

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

	var response managementapi.PluginSettingsResponse
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
	fakeRuntime := &capturingRuntime{events: make(chan pluginruntime.Event, 1)}
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

	var response managementapi.PluginSettingsUpdateResponse
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
	router.Delete("/api/plugins/{plugin_id}/secrets", handlers.handlePluginSecretsDelete())

	getRequest := httptest.NewRequest(http.MethodGet, "/api/plugins/example-config-panel/secrets", nil)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	var getResponse managementapi.PluginSecretsResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !getResponse.Configured["bili_token_primary"] {
		t.Fatalf("unexpected configured status: %#v", getResponse.Configured)
	}
	if _, exists := getResponse.Configured["other-plugin"]; exists {
		t.Fatalf("unexpected cross-plugin secret: %#v", getResponse.Configured)
	}

	body := bytes.NewReader([]byte(`{"values":{"bili_token_backup":"SESSDATA=backup"}}`))
	putRequest := httptest.NewRequest(http.MethodPut, "/api/plugins/example-config-panel/secrets", body)
	putRequest.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	router.ServeHTTP(putRecorder, putRequest)

	if putRecorder.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", putRecorder.Code, putRecorder.Body.String())
	}

	var putResponse managementapi.PluginSecretsUpdateResponse
	if err := json.Unmarshal(putRecorder.Body.Bytes(), &putResponse); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if len(putResponse.ChangedKeys) != 1 || putResponse.ChangedKeys[0] != "bili_token_backup" {
		t.Fatalf("unexpected changed_keys: %#v", putResponse.ChangedKeys)
	}
	if !putResponse.Configured["bili_token_backup"] {
		t.Fatalf("unexpected updated status: %#v", putResponse.Configured)
	}

	deleteBody := bytes.NewReader([]byte(`{"keys":["bili_token_primary"]}`))
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/plugins/example-config-panel/secrets", deleteBody)
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	var deleteResponse managementapi.PluginSecretsUpdateResponse
	if err := json.Unmarshal(deleteRecorder.Body.Bytes(), &deleteResponse); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if deleteResponse.Configured["bili_token_primary"] {
		t.Fatalf("deleted secret still configured: %#v", deleteResponse.Configured)
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
