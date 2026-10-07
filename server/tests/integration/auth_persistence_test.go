package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/coder/websocket"
)

func TestBootstrapStateAndBootstrapTokenSurviveRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "persist-a", withPersistentBridgeDispatch())

	setupFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))
	edgeFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "edge.setup-admin-already-initialized.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, appA, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}
	bootstrapToken := decodeBody(t, setup.Body.Bytes())["session_token"].(string)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "persist-b", withPersistentBridgeDispatch())
	defer closePersistentTestApp(t, appB)

	repeatSetup := performJSONRequest(t, appB, edgeFixture.Request.Method, edgeFixture.Request.Path, edgeFixture.Request.Body)
	if repeatSetup.Code != edgeFixture.Response.Status {
		t.Fatalf("unexpected repeated bootstrap status: got %d want %d", repeatSetup.Code, edgeFixture.Response.Status)
	}
	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, repeatSetup.Body.Bytes()), edgeFixture.Response.Body, "permission.denied")

	login := performJSONRequest(t, appB, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status after restart: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	server := newManagementTestServer(t, appB.Handler())
	defer server.Close()
	conn := dialEventsWebSocket(t, server.URL, bootstrapToken)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(1000, "") }(conn.Close)
}

func TestLoginTokenSurvivesRestartAndReceivesEvents(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time {
		return current
	}, "ws-a", withPersistentBridgeDispatch())

	setupFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.session-login.yaml"))

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
	}, "ws-b", withPersistentBridgeDispatch())
	defer closePersistentTestApp(t, appB)

	server := newManagementTestServer(t, appB.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, loginToken)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(1000, "") }(conn.Close)

	eventBridge := appB.Bridge()
	waitForObservabilitySubscriber(t, eventBridge)
	if outcome := eventBridge.HandleAdapterEvent(context.Background(), testBridgeEvent()); outcome != chatevent.DeliveryOutcomeDelivered {
		t.Fatalf("unexpected bridge outcome after restart: got %q want %q", outcome, chatevent.DeliveryOutcomeDelivered)
	}

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(readCtx); err != nil {
		t.Fatalf("expected persisted login token websocket to receive frame, got %v", err)
	}
}

func newPersistentTestApp(t *testing.T, configPath string, now func() time.Time, sessionPrefix string, configureOptions ...func(*app.Options)) *app.App {
	t.Helper()

	sessionCounter := 0
	repoRoot := newPreparedTestRuntimeRoot(t)
	options := app.Options{
		ConfigPath:           configPath,
		SchemaPath:           testutil.RepoPath(t, "contracts", "config.user.schema.json"),
		SetupToken:           testutil.TestSetupToken,
		LauncherControlToken: testutil.TestLauncherControlToken,
		PluginRepoRoot:       repoRoot,
		PluginSchemaPath:     testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugincatalog.ScanRoot{
			{Label: "plugins/installed", Path: filepath.Join(repoRoot, "plugins", "installed")},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
		AuthOptions: []auth.Option{
			auth.WithClock(now),
			auth.WithTokenGenerator(func() (string, error) {
				sessionCounter++
				return sessionPrefix + "-" + string(rune('0'+sessionCounter)), nil
			}),
		},
	}
	for _, configure := range configureOptions {
		configure(&options)
	}
	application, err := app.New(options)
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

// withPersistentBridgeDispatch injects a deliverable dispatch stub so bridge
// runtime frames flow without a real plugin runtime.
func withPersistentBridgeDispatch() func(*app.Options) {
	return func(options *app.Options) {
		options.BridgeDispatch = &persistentDispatchStub{
			deliverable: true,
			results: []dispatch.DeliveryResult{{
				PluginID: "weather",
				Outcome:  dispatch.OutcomeDelivered,
			}},
		}
	}
}

type persistentDispatchStub struct {
	deliverable bool
	results     []dispatch.DeliveryResult
}

func (s *persistentDispatchStub) HasDeliverablePlugins() bool {
	return s.deliverable
}

func (s *persistentDispatchStub) Dispatch(context.Context, chatevent.Event, string) []dispatch.DeliveryResult {
	return append([]dispatch.DeliveryResult(nil), s.results...)
}
