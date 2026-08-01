package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

const (
	testManagementAuthority = testutil.TestManagementAuthority
	testManagementOrigin    = testutil.TestManagementOrigin
)

func TestMain(m *testing.M) {
	if err := os.Chdir(testutil.ResolveRepoPath("server")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func newTestApp(t *testing.T, authOptions ...auth.Option) *app.App {
	application, _, _ := newTestAppWithOptions(t, nil, nil, authOptions...)
	return application
}

func newManagementTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	return testutil.NewManagementTestServer(t, handler)
}

func newTestAppWithConfigMutation(t *testing.T, mutate func(map[string]any), authOptions ...auth.Option) (*app.App, string, string) {
	return newTestAppWithOptions(t, mutate, nil, authOptions...)
}

func newTestAppWithOptions(
	t *testing.T,
	mutate func(map[string]any),
	configureOptions func(*app.Options, string),
	authOptions ...auth.Option,
) (*app.App, string, string) {
	t.Helper()

	fixture := testutil.LoadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

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

	configPath := testutil.WriteYAMLConfig(t, updated)
	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
	repoRoot := testutil.NewPreparedTestRuntimeRoot(t)
	builtinRoot := testutil.WriteEchoGoPluginArtifact(t, repoRoot)

	options := app.Options{
		ConfigPath:           configPath,
		SchemaPath:           schemaPath,
		SetupToken:           testutil.TestSetupToken,
		LauncherControlToken: testutil.TestLauncherControlToken,
		PluginRepoRoot:       repoRoot,
		PluginSchemaPath:     testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugincatalog.ScanRoot{
			{Label: "plugins/builtin", Path: builtinRoot},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
		AuthOptions: authOptions,
	}
	if configureOptions != nil {
		configureOptions(&options, configPath)
	}

	application, err := app.New(options)
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

func deterministicAuthOptions() []auth.Option {
	return testutil.DeterministicAuthOptions()
}

func loadConfigFixture(t *testing.T, path string) testutil.ConfigFixture {
	return testutil.LoadConfigFixture(t, path)
}

func writeYAMLConfig(t *testing.T, raw json.RawMessage) string {
	return testutil.WriteYAMLConfig(t, raw)
}

func writePersistentYAMLConfig(t *testing.T, databasePath string) string {
	return testutil.WritePersistentYAMLConfig(t, databasePath)
}

func newPersistentTestApp(t *testing.T, configPath string, now func() time.Time, sessionPrefix string) *app.App {
	t.Helper()

	sessionCounter := 0
	repoRoot := testutil.NewPreparedTestRuntimeRoot(t)
	builtinRoot := testutil.WriteEchoGoPluginArtifact(t, repoRoot)
	application, err := app.New(app.Options{
		ConfigPath:           configPath,
		SchemaPath:           testutil.RepoPath(t, "contracts", "config.user.schema.json"),
		SetupToken:           testutil.TestSetupToken,
		LauncherControlToken: testutil.TestLauncherControlToken,
		PluginRepoRoot:       repoRoot,
		PluginSchemaPath:     testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugincatalog.ScanRoot{
			{Label: "plugins/builtin", Path: builtinRoot},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
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

func issueExistingBootstrapLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	return testutil.IssueExistingBootstrapLoginToken(t, application)
}

func loadWebAPIFixtureDocument(t *testing.T, path string) testutil.WebAPIFixtureDocument {
	return testutil.LoadWebAPIFixtureDocument(t, path)
}

func performJSONRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any) *httptest.ResponseRecorder {
	return testutil.PerformJSONRequest(t, application, method, path, body)
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	return testutil.DecodeBody(t, raw)
}

func readAll(t *testing.T, response *http.Response) []byte {
	return testutil.ReadAll(t, response)
}
