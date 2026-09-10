package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestReadyzReportsActualMissingTemplateDependency(t *testing.T) {
	t.Parallel()
	for _, prepared := range []bool{true, false} {
		name := "missing"
		if prepared {
			name = "prepared"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := testutil.NewPreparedTestRuntimeRoot(t)
			if !prepared {
				if err := os.Remove(filepath.Join(root, "templates", "help.menu", "template.json")); err != nil {
					t.Fatal(err)
				}
			}
			application, err := app.New(app.Options{
				ConfigPath:       testutil.WritePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db")),
				SchemaPath:       testutil.RepoPath(t, "contracts", "config.user.schema.json"),
				PluginRepoRoot:   root,
				PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
				PluginRoots:      []catalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Join(root, "plugins", "installed")}},
				SetupToken:       testutil.TestSetupToken,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := application.Close(); err != nil {
					t.Error(err)
				}
			})
			_ = testutil.IssueLoginToken(t, application)
			recorder := httptest.NewRecorder()
			application.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			var report struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
				Issues []struct {
					Code string `json:"code"`
				} `json:"issues"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if prepared {
				if recorder.Code != http.StatusOK || report.Status != "ready" || report.Checks["render"] != "ok" {
					t.Fatalf("prepared template: %d %s", recorder.Code, recorder.Body.String())
				}
				return
			}
			if recorder.Code != http.StatusOK || report.Status != "degraded" || report.Checks["render"] != "resource_missing" {
				t.Fatalf("missing template: %d %s", recorder.Code, recorder.Body.String())
			}
			for _, issue := range report.Issues {
				if issue.Code == "platform.resource_missing" {
					return
				}
			}
			t.Fatalf("missing template did not produce actionable issue: %s", recorder.Body.String())
		})
	}
}
