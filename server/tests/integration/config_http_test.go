package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	secretssqlite "github.com/RayleaBot/RayleaBot/server/internal/platform/secrets/sqlite"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestConfigGetRedactsOneBotTransportTokens(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		onebot := testutil.ConfigDocumentOneBot(t, input)
		onebot["forward_ws"].(map[string]any)["access_token"] = "forward-secret"
		onebot["reverse_ws"].(map[string]any)["access_token"] = "reverse-secret"
		onebot["http_api"].(map[string]any)["access_token"] = "http-secret"
		onebot["webhook"].(map[string]any)["access_token"] = "webhook-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.config-get-response.yaml"))
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create config get request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform config get request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected config get status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	expected := normalizeJSONMap(t, fixture.Response.Body)
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected config get body: got %#v want %#v", body, expected)
	}
}

func TestConfigPutWritesValidatedDocumentAndRedactsTransportTokens(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		testutil.ConfigDocumentOneBot(t, input)["forward_ws"].(map[string]any)["access_token"] = "old-forward-secret"
	}, deterministicAuthOptions()...)
	initialPort := application.CurrentConfig().Server.Port
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.config-update-response.yaml"))
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	updateRequest := normalizeJSONMap(t, fixture.Request.Body)
	payload, err := json.Marshal(updateRequest)
	if err != nil {
		t.Fatalf("marshal config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, server.URL+fixture.Request.Path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform config update request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected config update status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	expected := normalizeJSONMap(t, fixture.Response.Body)
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected config update body: got %#v want %#v", body, expected)
	}

	document, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if got := document["server"].(map[string]any)["port"]; got != float64(8081) {
		t.Fatalf("unexpected persisted server.port: got %#v want 8081", got)
	}
	if got := document["log"].(map[string]any)["level"]; got != "debug" {
		t.Fatalf("unexpected persisted log.level: got %#v want debug", got)
	}
	onebot := testutil.ConfigDocumentOneBot(t, document)
	if got := onebot["forward_ws"].(map[string]any)["access_token"]; got != forwardTokenReference {
		t.Fatalf("unexpected persisted forward_ws.access_token: got %#v want secret reference", got)
	}
	assertStoredConfigSecret(t, application, forwardTokenStoreKey, "forward-secret")

	if application.CurrentConfig().Server.Port != initialPort {
		t.Fatalf("restart-only server.port changed before restart: got %d want %d", application.CurrentConfig().Server.Port, initialPort)
	}
	if application.CurrentConfig().Log.Level != "debug" {
		t.Fatalf("expected live config log.level to be hot-reloaded to debug, got %q", application.CurrentConfig().Log.Level)
	}
	if got := liveOneBot(t, application).ForwardWS.AccessToken; got != "forward-secret" {
		t.Fatalf("expected live config forward token to be resolved, got %q", got)
	}
}

func TestAppNewResolvesOneBotSecretReferences(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithOptions(t, func(input map[string]any) {
		testutil.ConfigDocumentOneBot(t, input)["forward_ws"].(map[string]any)["access_token"] = forwardTokenReference
	}, func(_ *internalapp.Options, configPath string) {
		storeConfigSecretFixture(t, configPath, forwardTokenStoreKey, "startup-secret")
	}, deterministicAuthOptions()...)

	if got := liveOneBot(t, application).ForwardWS.AccessToken; got != "startup-secret" {
		t.Fatalf("forward access token = %q, want startup-secret", got)
	}
}

func TestConfigPutRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		testutil.ConfigDocumentOneBot(t, input)["forward_ws"].(map[string]any)["access_token"] = "fixture-only-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "invalid.config-update-invalid.yaml"))
	before, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load baseline config: %v", err)
	}

	payload, err := json.Marshal(fixture.Request.Body)
	if err != nil {
		t.Fatalf("marshal invalid config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create invalid config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Host = testManagementAuthority
	recorder := httptest.NewRecorder()

	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected invalid config update status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.invalid_config")
	if strings.Contains(recorder.Body.String(), "fixture-only-secret") {
		t.Fatalf("invalid config response leaked secret content: %s", recorder.Body.String())
	}

	after, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("reload config after invalid update: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("config file changed after invalid update: got %#v want %#v", after, before)
	}
}

// The OneBot adapter of the config fixture keys its secrets under its own
// instance id, so its reference and store key are spelled once here.
var (
	forwardTokenPath      = []string{"adapters", internalconfig.DefaultOneBot11AdapterID, "onebot11", "forward_ws", "access_token"}
	forwardTokenReference = internalconfig.SecretReferenceFor(forwardTokenPath)
	forwardTokenStoreKey  = internalconfig.SecretStoreKeyFor(forwardTokenPath)
)

func liveOneBot(t *testing.T, application *internalapp.App) internalconfig.OneBotConfig {
	t.Helper()
	settings, ok := application.CurrentConfig().OneBot11Settings(internalconfig.DefaultOneBot11AdapterID)
	if !ok {
		t.Fatal("live config has no onebot11 adapter")
	}
	return settings
}

func newTestAppWithConfigMutation(t *testing.T, mutate func(map[string]any), authOptions ...auth.Option) (*internalapp.App, string, string) {
	return newTestAppWithOptions(t, mutate, nil, authOptions...)
}

func assertStoredConfigSecret(t *testing.T, application *internalapp.App, key string, want string) {
	t.Helper()
	secretStore, err := secretssqlite.NewStore(application.Storage())
	if err != nil {
		t.Fatalf("create sqlite secret store: %v", err)
	}
	stored, err := secretStore.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("read config secret %s: %v", key, err)
	}
	if string(stored) != want {
		t.Fatalf("config secret %s = %q, want %q", key, stored, want)
	}
}

func storeConfigSecretFixture(t *testing.T, configPath string, key string, value string) {
	t.Helper()
	dbPath, err := runtimepaths.ResolveDatabasePath(configPath, "data/rayleabot.db")
	if err != nil {
		t.Fatalf("resolve database path: %v", err)
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)
	secretStore, err := secretssqlite.NewStore(store)
	if err != nil {
		t.Fatalf("create sqlite secret store: %v", err)
	}
	if err := secretStore.Set(context.Background(), key, []byte(value)); err != nil {
		t.Fatalf("store config secret fixture: %v", err)
	}
}

func newTestAppWithOptions(
	t *testing.T,
	mutate func(map[string]any),
	configureOptions func(*internalapp.Options, string),
	authOptions ...auth.Option,
) (*internalapp.App, string, string) {
	t.Helper()

	fixture := loadConfigFixture(t, testutil.RepoPath(t, "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}
	if mutate != nil {
		mutate(input)
	}

	updated, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal config fixture input: %v", err)
	}

	configPath := writeYAMLConfig(t, updated)
	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
	repoRoot := newPreparedTestRuntimeRoot(t)
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")

	options := internalapp.Options{
		ConfigPath:           configPath,
		SchemaPath:           schemaPath,
		SetupToken:           testutil.TestSetupToken,
		LauncherControlToken: testutil.TestLauncherControlToken,
		PluginRepoRoot:       repoRoot,
		PluginSchemaPath:     testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugincatalog.ScanRoot{
			{Label: "plugins/installed", Path: installedRoot},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
		AuthOptions: authOptions,
	}
	if configureOptions != nil {
		configureOptions(&options, configPath)
	}

	application, err := internalapp.New(options)
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application, configPath, schemaPath
}

func normalizeJSONMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal expected body: %v", err)
	}

	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		t.Fatalf("unmarshal expected body: %v", err)
	}

	return normalized
}
