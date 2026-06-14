package integration

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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
	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "ok.healthz-response.yaml"))

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

	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "edge.readyz-degraded-response.yaml"))
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

	report := health.ReadinessReport{
		Status:      fixture.Response.Body["status"].(string),
		Reason:      fixture.Response.Body["reason"].(string),
		ReasonCodes: toStringSlice(fixture.Response.Body["reason_codes"].([]any)),
		Checks:      checks,
		Issues:      issues,
	}
	if rawSummary, ok := fixture.Response.Body["recovery_summary"].(map[string]any); ok {
		report.RecoverySummary = &recovery.CompatibilitySummary{
			Status:                    rawSummary["status"].(string),
			Phase:                     rawSummary["phase"].(string),
			Operation:                 rawSummary["operation"].(string),
			CreatedAt:                 fmt.Sprint(rawSummary["created_at"]),
			UpdatedAt:                 fmt.Sprint(rawSummary["updated_at"]),
			SourceCoreVersion:         rawSummary["source_core_version"].(string),
			TargetCoreVersion:         rawSummary["target_core_version"].(string),
			SourceConfigSchemaVersion: rawSummary["source_config_schema_version"].(string),
			TargetConfigSchemaVersion: rawSummary["target_config_schema_version"].(string),
			SourceDBSchemaVersion:     rawSummary["source_db_schema_version"].(string),
			TargetDBSchemaVersion:     rawSummary["target_db_schema_version"].(string),
			ManualActions:             toStringSlice(rawSummary["manual_actions"].([]any)),
			NextSteps:                 toStringSlice(rawSummary["next_steps"].([]any)),
		}
		if rawIssues, ok := rawSummary["issues"].([]any); ok {
			for _, raw := range rawIssues {
				item := raw.(map[string]any)
				report.RecoverySummary.Issues = append(report.RecoverySummary.Issues, recovery.CompatibilityIssue{
					Code:        item["code"].(string),
					Severity:    item["severity"].(string),
					Summary:     item["summary"].(string),
					Remediation: item["remediation"].(string),
				})
			}
		}
		if rawSkipped, ok := rawSummary["skipped_plugins"].([]any); ok {
			for _, raw := range rawSkipped {
				item := raw.(map[string]any)
				skipped := recovery.SkippedPlugin{
					PluginID:     item["plugin_id"].(string),
					ReasonCode:   item["reason_code"].(string),
					Summary:      item["summary"].(string),
					ReviewID:     item["review_id"].(string),
					ReviewStatus: item["review_status"].(string),
					ManualAction: item["manual_action"].(string),
				}
				if version, ok := item["version"].(string); ok {
					skipped.Version = version
				}
				if reviewedAt, ok := item["reviewed_at"].(string); ok {
					skipped.ReviewedAt = reviewedAt
				}
				if reviewedBy, ok := item["reviewed_by"].(string); ok {
					skipped.ReviewedBy = reviewedBy
				}
				report.RecoverySummary.SkippedPlugins = append(report.RecoverySummary.SkippedPlugins, skipped)
			}
		}
		if rawAudit, ok := rawSummary["audit"].([]any); ok {
			for _, raw := range rawAudit {
				item := raw.(map[string]any)
				entry := recovery.AuditEntry{
					TaskID:     item["task_id"].(string),
					CreatedAt:  item["created_at"].(string),
					OperatorID: item["operator_id"].(string),
					Note:       item["note"].(string),
				}
				if rawItems, ok := item["items"].([]any); ok {
					for _, rawItem := range rawItems {
						auditItem := rawItem.(map[string]any)
						record := recovery.AuditItem{
							ReviewID:   auditItem["review_id"].(string),
							PluginID:   auditItem["plugin_id"].(string),
							ReasonCode: auditItem["reason_code"].(string),
							Summary:    auditItem["summary"].(string),
						}
						if version, ok := auditItem["version"].(string); ok {
							record.Version = version
						}
						entry.Items = append(entry.Items, record)
					}
				}
				report.RecoverySummary.Audit = append(report.RecoverySummary.Audit, entry)
			}
		}
	}

	handler := health.NewReadinessHandler(func() health.ReadinessReport {
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
	fixture := loadWebAPIFixture(t, filepath.Join("..", "fixtures", "web-api", "edge.readyz-setup-required-response.yaml"))
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

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))
	configPath := writeYAMLConfig(t, fixture.Input)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")
	repoRoot := newPreparedTestRuntimeRoot(t)
	builtinRoot := filepath.Join(repoRootPath(t), "plugins", "builtin")

	application, err := app.New(app.Options{
		ConfigPath:       configPath,
		SchemaPath:       schemaPath,
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: filepath.Join("..", "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugindiscovery.ScanRoot{
			{Label: "plugins/builtin", Path: builtinRoot},
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
