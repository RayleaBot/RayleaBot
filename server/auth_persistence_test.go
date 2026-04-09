package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const sessionSigningKeySecret = "platform.auth.session_signing_key"

func TestBootstrapStateAndBootstrapTokenSurviveRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "persist-a")
	appA.Bridge = newPersistentEventsBridge(appA)

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	edgeFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.setup-admin-already-initialized.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, appA, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}
	bootstrapToken := decodeBody(t, setup.Body.Bytes())["session_token"].(string)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "persist-b")
	defer closePersistentTestApp(t, appB)
	appB.Bridge = newPersistentEventsBridge(appB)

	repeatSetup := performJSONRequest(t, appB, edgeFixture.Request.Method, edgeFixture.Request.Path, edgeFixture.Request.Body)
	if repeatSetup.Code != edgeFixture.Response.Status {
		t.Fatalf("unexpected repeated bootstrap status: got %d want %d", repeatSetup.Code, edgeFixture.Response.Status)
	}
	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, repeatSetup.Body.Bytes()), edgeFixture.Response.Body, "permission.denied")

	login := performJSONRequest(t, appB, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status after restart: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	server := httptest.NewServer(appB.Handler())
	defer server.Close()
	conn := dialEventsWebSocket(t, server.URL, bootstrapToken)
	defer conn.Close(1000, "")
}

func TestLoginTokenSurvivesRestartAndReceivesEvents(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "ws-a")
	appA.Bridge = newPersistentEventsBridge(appA)

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, appA, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}
	login := performJSONRequest(t, appA, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status: got %d want %d", login.Code, loginFixture.Response.Status)
	}
	loginToken := decodeBody(t, login.Body.Bytes())["session_token"].(string)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "ws-b")
	defer closePersistentTestApp(t, appB)
	appB.Bridge = newPersistentEventsBridge(appB)

	server := httptest.NewServer(appB.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, loginToken)
	defer conn.Close(1000, "")

	waitForObservabilitySubscriber(t, appB.Bridge)
	if outcome := appB.Bridge.HandleAdapterEvent(context.Background(), testBridgeEvent()); outcome != bridge.OutcomeDelivered {
		t.Fatalf("unexpected bridge outcome after restart: got %q want %q", outcome, bridge.OutcomeDelivered)
	}

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(readCtx); err != nil {
		t.Fatalf("expected persisted login token websocket to receive frame, got %v", err)
	}
}

func TestProductionAppPersistsSessionSigningKeyInSecretStore(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "state.db")
	configPath := writePersistentYAMLConfig(t, dbPath)
	application := newPersistentTestApp(t, configPath, time.Now, "secret-a")
	defer closePersistentTestApp(t, application)

	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("close sqlite store: %v", closeErr)
		}
	}()

	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("create sqlite secret store: %v", err)
	}

	signingKey, err := secretStore.Get(context.Background(), sessionSigningKeySecret)
	if err != nil {
		t.Fatalf("expected persisted session signing key, got %v", err)
	}
	if len(signingKey) == 0 {
		t.Fatalf("expected non-empty persisted session signing key")
	}
}

func TestDeletingPersistedSessionSigningKeyInvalidatesOlderTokens(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	dbPath := filepath.Join(t.TempDir(), "state.db")
	configPath := writePersistentYAMLConfig(t, dbPath)

	appA := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "secret-b")
	loginToken := issueLoginToken(t, appA)
	closePersistentTestApp(t, appA)

	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("close sqlite store: %v", closeErr)
		}
	}()

	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("create sqlite secret store: %v", err)
	}
	if err := secretStore.Delete(context.Background(), sessionSigningKeySecret); err != nil {
		t.Fatalf("delete persisted session signing key: %v", err)
	}

	appB := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "secret-c")
	defer closePersistentTestApp(t, appB)

	server := httptest.NewServer(appB.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/events?session_token="+loginToken, nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatalf("expected websocket dial to fail after deleting persisted signing key")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		if response == nil {
			t.Fatalf("expected unauthorized response, got nil")
		}
		t.Fatalf("unexpected unauthorized status: got %d want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func newPersistentTestApp(t *testing.T, configPath string, now func() time.Time, sessionPrefix string) *app.App {
	t.Helper()

	sessionCounter := 0
	application, err := app.New(app.Options{
		ConfigPath: configPath,
		SchemaPath: filepath.Join("..", "contracts", "config.user.schema.json"),
		AuthOptions: []auth.Option{
			auth.WithClock(now),
			auth.WithSessionIDGenerator(func() (string, error) {
				sessionCounter++
				return sessionPrefix + "-" + string(rune('0'+sessionCounter)), nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}

	return application
}

func closePersistentTestApp(t *testing.T, application *app.App) {
	t.Helper()

	if application != nil {
		if err := application.Close(); err != nil {
			t.Fatalf("close persistent app resources: %v", err)
		}
	}
}

func newPersistentEventsBridge(application *app.App) *bridge.Bridge {
	return bridge.New(application.Logger, &eventsDispatchStub{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "weather",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	})
}

func writePersistentYAMLConfig(t *testing.T, databasePath string) string {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}

	database := input["database"].(map[string]any)
	database["path"] = databasePath

	yamlBytes, err := yaml.Marshal(input)
	if err != nil {
		t.Fatalf("marshal persistent yaml: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(configPath, yamlBytes, 0o644); err != nil {
		t.Fatalf("write persistent config: %v", err)
	}

	return configPath
}
