package integration

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

type webAPIFixture struct {
	Response struct {
		Status int            `yaml:"status"`
		Body   map[string]any `yaml:"body"`
	} `yaml:"response"`
}

func TestHealthzResponseMatchesFixture(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixture(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.healthz-response.yaml"))

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

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected /healthz body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestReadinessHandlerEncodesDegradedFixtureShape(t *testing.T) {
	t.Parallel()

	fixture := loadWebAPIFixture(t, testutil.RepoPath(t, "fixtures", "web-api", "edge.readyz-degraded-response.yaml"))
	checks := map[string]string{}
	for key, value := range fixture.Response.Body["checks"].(map[string]any) {
		checks[key] = value.(string)
	}

	var issues []health.DiagnosticIssue
	if rawIssues, ok := fixture.Response.Body["issues"].([]any); ok {
		for _, raw := range rawIssues {
			m := raw.(map[string]any)
			issue := health.DiagnosticIssue{
				Code:     m["code"].(string),
				Severity: m["severity"].(string),
				Summary:  m["summary"].(string),
			}
			if rem, ok := m["remediation"].(string); ok {
				issue.Remediation = rem
			}
			issues = append(issues, issue)
		}
	}

	report := systemsvc.ReadinessReport{
		Status:      fixture.Response.Body["status"].(string),
		Reason:      fixture.Response.Body["reason"].(string),
		ReasonCodes: toStringSlice(fixture.Response.Body["reason_codes"].([]any)),
		Checks:      checks,
		Issues:      issues,
	}
	handler := managementapi.NewReadinessHandler(func() systemsvc.ReadinessReport {
		return report
	})

	request := httptest.NewRequest("GET", "/readyz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected degraded status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal degraded body: %v", err)
	}

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected degraded body: got %#v want %#v", body, fixture.Response.Body)
	}
}

func TestReadyzReportsSetupRequiredBeforeBootstrap(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	fixture := loadWebAPIFixture(t, testutil.RepoPath(t, "fixtures", "web-api", "edge.readyz-setup-required-response.yaml"))
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

	if !reflect.DeepEqual(body, fixture.Response.Body) {
		t.Fatalf("unexpected setup-required body: got %#v want %#v", body, fixture.Response.Body)
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

func loadWebAPIFixture(t *testing.T, path string) webAPIFixture {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture webAPIFixture
	if err := yaml.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}
	fixture.Response.Body = normalizeFixtureMap(fixture.Response.Body)

	return fixture
}

func toStringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.(string))
	}

	return result
}

func normalizeFixtureMap(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = normalizeFixtureValue(value)
	}
	return result
}

func normalizeFixtureValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeFixtureMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeFixtureValue(item))
		}
		return items
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return value
	}
}
