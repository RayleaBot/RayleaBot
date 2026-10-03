package integration

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestHealthzResponseMatchesFixture(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.healthz-response.yaml"))

	request := httptest.NewRequest("GET", "/healthz", nil)
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal /healthz body: %v", err)
	}

	if expected := normalizeJSONMap(t, fixture.Response.Body); !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected /healthz body: got %#v want %#v", body, expected)
	}
}

func TestReadyzReportsSetupRequiredBeforeBootstrap(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "edge.readyz-setup-required-response.yaml"))
	request := httptest.NewRequest("GET", "/readyz", nil)
	recorder := httptest.NewRecorder()

	application.Handler().ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected setup-required status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal setup-required body: %v", err)
	}

	if expected := normalizeJSONMap(t, fixture.Response.Body); !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected setup-required body: got %#v want %#v", body, expected)
	}
}

func newTestApp(t *testing.T, authOptions ...auth.Option) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, testutil.RepoPath(t, "fixtures", "config", "ok.minimal.json"))
	configPath := writeYAMLConfig(t, fixture.Input)
	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
	repoRoot := newPreparedTestRuntimeRoot(t)
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")

	application, err := app.New(app.Options{
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
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application
}
