package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestHandleSystemRecoveryRecheckAcceptsTaskAndPersistsCompatibleSummary(t *testing.T) {
	repoRoot := t.TempDir()
	writeDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	writePreparedRuntime(t, repoRoot, "python-"+platform, "3.12.13", "python", "python.exe")
	writePreparedRuntime(t, repoRoot, "python-"+platform, "3.12.13", "python", "Scripts", "pip.exe")
	writePreparedRuntime(t, repoRoot, "nodejs-"+platform, "24.14.0", "node-v24.14.0-win-x64", "node.exe")
	writePreparedRuntime(t, repoRoot, "nodejs-"+platform, "24.14.0", "node-v24.14.0-win-x64", "npm.cmd")
	if err := recovery.SaveSummary(repoRoot, recovery.CompatibilitySummary{
		Status:            "degraded",
		Phase:             "post_startup",
		Operation:         "upgrade",
		CreatedAt:         "2026-04-03T08:00:00Z",
		UpdatedAt:         "2026-04-03T08:00:01Z",
		TargetCoreVersion: "0.2.0",
		ManualActions:     []string{"stale action"},
		NextSteps:         []string{"stale step"},
		SkippedPlugins:    []recovery.SkippedPlugin{{PluginID: "stale-plugin"}},
	}); err != nil {
		t.Fatalf("save recovery summary: %v", err)
	}

	application := newTaskOnlyApp(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/recheck", nil)
	recorder := httptest.NewRecorder()

	application.handleSystemRecoveryRecheck().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := waitTask(t, application.tasks, accepted.TaskID, tasks.StatusSucceeded)
	if snapshot.TaskType != "recovery.recheck" {
		t.Fatalf("unexpected task type: %#v", snapshot)
	}

	summary, err := recovery.LoadSummary(repoRoot)
	if err != nil {
		t.Fatalf("load recovery summary: %v", err)
	}
	if summary == nil || summary.Status != "compatible" {
		t.Fatalf("expected compatible recovery summary, got %#v", summary)
	}
	if len(summary.ManualActions) != 0 || len(summary.NextSteps) != 0 || len(summary.SkippedPlugins) != 0 {
		t.Fatalf("expected compatible summary to clear operator guidance, got %#v", summary)
	}
}

func TestHandleSystemRecoveryRecheckRejectsMissingSummary(t *testing.T) {
	application := newTaskOnlyApp(t, t.TempDir())
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/recheck", nil)
	recorder := httptest.NewRecorder()

	application.handleSystemRecoveryRecheck().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload["error"].(map[string]any)["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestHandleSystemRecoveryConfirmAcceptsTaskAndPersistsAudit(t *testing.T) {
	repoRoot := t.TempDir()
	initial := recovery.Finalize(
		recovery.CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
		},
		recovery.FinalizeInput{
			Readiness: recovery.RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{{
				PluginID:          "weather-pro",
				Version:           "1.4.0",
				MinCoreVersion:    "0.3.0",
				ManifestPath:      "plugins/installed/weather-pro/info.json",
				SourceRoot:        "plugins/installed",
				RegistrationState: "installed",
				DesiredState:      "disabled",
			}},
		},
	)
	if err := recovery.SaveSummary(repoRoot, initial); err != nil {
		t.Fatalf("save recovery summary: %v", err)
	}

	application := newTaskOnlyApp(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/confirm", stringsNewReader(`{"review_ids":["`+initial.SkippedPlugins[0].ReviewID+`"],"note":"已确认当前跳过状态。"}`))
	request = request.WithContext(context.WithValue(request.Context(), claimsKey{}, auth.Claims{Subject: "alice"}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	application.handleSystemRecoveryConfirm().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202 (%s)", recorder.Code, recorder.Body.String())
	}

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := waitTask(t, application.tasks, accepted.TaskID, tasks.StatusSucceeded)
	if snapshot.TaskType != "recovery.confirm" {
		t.Fatalf("unexpected task type: %#v", snapshot)
	}
	if snapshot.Result == nil {
		t.Fatalf("expected task result, got %#v", snapshot)
	}
	confirmedReviewIDs, ok := snapshot.Result.Details["confirmed_review_ids"].([]string)
	if !ok || !slices.Equal(confirmedReviewIDs, []string{initial.SkippedPlugins[0].ReviewID}) {
		t.Fatalf("unexpected confirmed review ids: %#v", snapshot.Result.Details["confirmed_review_ids"])
	}

	summary, err := recovery.LoadSummary(repoRoot)
	if err != nil {
		t.Fatalf("load recovery summary: %v", err)
	}
	if summary == nil || summary.Status != "compatible" {
		t.Fatalf("expected compatible recovery summary, got %#v", summary)
	}
	if len(summary.SkippedPlugins) != 1 || summary.SkippedPlugins[0].ReviewStatus != "confirmed" {
		t.Fatalf("expected confirmed skipped plugin state, got %#v", summary.SkippedPlugins)
	}
	if len(summary.Audit) != 1 || summary.Audit[0].TaskID != accepted.TaskID || summary.Audit[0].OperatorID != "alice" {
		t.Fatalf("expected persisted audit entry, got %#v", summary.Audit)
	}
}

func TestHandleSystemRecoveryConfirmRejectsUnknownReviewID(t *testing.T) {
	repoRoot := t.TempDir()
	if err := recovery.SaveSummary(repoRoot, recovery.CompatibilitySummary{
		Status:    "degraded",
		Phase:     "post_startup",
		Operation: "upgrade",
		CreatedAt: "2026-04-04T08:00:00Z",
		UpdatedAt: "2026-04-04T08:00:00Z",
		SkippedPlugins: []recovery.SkippedPlugin{{
			PluginID:     "weather-pro",
			ReasonCode:   "plugin.min_core_version",
			Summary:      "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
			ReviewID:     "review_known",
			ReviewStatus: "pending",
		}},
	}); err != nil {
		t.Fatalf("save recovery summary: %v", err)
	}

	application := newTaskOnlyApp(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/confirm", stringsNewReader(`{"review_ids":["review_missing"]}`))
	request = request.WithContext(context.WithValue(request.Context(), claimsKey{}, auth.Claims{Subject: "alice"}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	application.handleSystemRecoveryConfirm().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want 400", recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload["error"].(map[string]any)["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
}

func TestHandleSystemRuntimeBootstrapAcceptsTaskAndReportsPreparedStoreHits(t *testing.T) {
	repoRoot := t.TempDir()
	writeDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	writePreparedRuntime(t, repoRoot, "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe")
	writePreparedRuntime(t, repoRoot, "python-"+platform, "3.12.13", "python", "python.exe")
	writePreparedRuntime(t, repoRoot, "python-"+platform, "3.12.13", "python", "Scripts", "pip.exe")

	application := newTaskOnlyApp(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/runtime/bootstrap", stringsNewReader(`{"resources":["chromium","python-runtime"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	application.handleSystemRuntimeBootstrap().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}
	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := waitTask(t, application.tasks, accepted.TaskID, tasks.StatusSucceeded)
	if snapshot.TaskType != "runtime.bootstrap" {
		t.Fatalf("unexpected task type: %#v", snapshot)
	}
	if snapshot.Result == nil {
		t.Fatalf("expected task result, got %#v", snapshot)
	}
	resources, ok := snapshot.Result.Details["resources"].([]any)
	if !ok || len(resources) != 2 {
		t.Fatalf("unexpected runtime bootstrap resources: %#v", snapshot.Result.Details)
	}
	first, ok := resources[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected runtime bootstrap result item: %#v", resources[0])
	}
	if _, ok := first["attempted_sources"]; !ok {
		t.Fatalf("runtime bootstrap result should expose attempted_sources: %#v", first)
	}
	if _, ok := first["selected_source"]; !ok {
		t.Fatalf("runtime bootstrap result should expose selected_source: %#v", first)
	}
}

func TestManagedRuntimeTaskProgressSummarizesSourceProbe(t *testing.T) {
	percent, summary := managedRuntimeTaskProgress(1, 0, deps.PrepareProgress{
		Kind:     "nodejs-runtime",
		Label:    deps.ManagedResourceLabel("nodejs-runtime"),
		Stage:    "probe",
		Status:   "running",
		Progress: 0,
	})

	if percent != 0 {
		t.Fatalf("unexpected probe percent: got %d want 0", percent)
	}
	if summary != "正在测试 Node.js / npm 环境下载来源" {
		t.Fatalf("unexpected probe summary: %q", summary)
	}
}

func TestHandleSystemRuntimeBootstrapRefreshesChromiumDiagnostics(t *testing.T) {
	repoRoot := t.TempDir()
	writeDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	store, err := storage.Open(filepath.Join(repoRoot, "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	renderer, err := render.NewService(render.Options{
		RepoRoot:   repoRoot,
		OutputRoot: filepath.Join(repoRoot, "render-out"),
		Store:      store,
	})
	if err != nil {
		t.Fatalf("create render service: %v", err)
	}
	t.Cleanup(func() {
		_ = renderer.Close()
	})

	application := newTaskOnlyApp(t, repoRoot)
	application.renderer = renderer
	application.systemService.renderer = renderer

	original := prepareManagedRuntimeWithProgress
	t.Cleanup(func() {
		prepareManagedRuntimeWithProgress = original
	})
	prepareManagedRuntimeWithProgress = func(_ context.Context, _ string, kind string, progress deps.PrepareProgressReporter) (*managedRuntimePrepareReport, error) {
		if progress != nil {
			progress(deps.PrepareProgress{
				Kind:     kind,
				Label:    deps.ManagedResourceLabel(kind),
				Stage:    "complete",
				Status:   "succeeded",
				Progress: 100,
				Summary:  deps.ManagedResourceLabel(kind) + "已准备完成",
			})
		}
		writePreparedRuntime(t, repoRoot, "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe")
		return &managedRuntimePrepareReport{
			Kind:               kind,
			ArchivePath:        filepath.Join(repoRoot, "cache", "downloads", "runtime", "chromium-"+platform+"-147.0.7727.24.zip"),
			StoreRoot:          filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "147.0.7727.24"),
			UsedPreparedStore:  false,
			UsedCachedArchive:  false,
			PreparedEntrypoint: filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe"),
		}, nil
	}

	if !containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected pre-bootstrap render diagnostics to warn about missing chromium")
	}

	request := httptest.NewRequest(http.MethodPost, "/api/system/runtime/bootstrap", stringsNewReader(`{"resources":["chromium"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	application.handleSystemRuntimeBootstrap().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	waitTask(t, application.tasks, accepted.TaskID, tasks.StatusSucceeded)

	if containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected runtime bootstrap to refresh chromium diagnostics")
	}
}

func newTaskOnlyApp(t *testing.T, repoRoot string) *App {
	t.Helper()
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 2*time.Second)
	t.Cleanup(func() {
		_ = executor.Close()
	})
	application := newTestAppState(config.Config{}, nil)
	application.state.repoRoot = repoRoot
	application.state.startedAt = time.Now()
	application.tasks = registry
	application.taskExecutor = executor
	application.plugins = plugins.NewCatalog(nil)
	application.setTestSystem(registry, executor, nil, nil)
	return application
}

func writeDepsManifest(t *testing.T, repoRoot string) {
	t.Helper()
	platform := deps.CurrentPlatform()
	chromiumID := "chromium-" + platform
	pythonID := "python-" + platform
	nodeID := "nodejs-" + platform
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "` + chromiumID + `",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    },
    {
      "id": "` + pythonID + `",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"],
        "pip": ["python/Scripts/pip.exe"]
      }
    },
    {
      "id": "` + nodeID + `",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + platform + `",
      "sources": [
        {
          "url": "https://example.invalid/node.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node-v24.14.0-win-x64/node.exe"],
        "npm": ["node-v24.14.0-win-x64/npm.cmd"]
      }
    }
  ]
}`
	path := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir deps manifest root: %v", err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "templates", "help.menu"), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "help.menu", "template.html"), []byte("<html><body>{{ .title }}</body></html>"), 0o644); err != nil {
		t.Fatalf("write template html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "help.menu", "template.json"), []byte(`{"id":"help.menu","version":"1","entry_html":"template.html","stylesheet":"styles.css","input_schema":"input.schema.json","width":960,"height":640}`), 0o644); err != nil {
		t.Fatalf("write template json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "help.menu", "styles.css"), []byte("body { color: #111; }"), 0o644); err != nil {
		t.Fatalf("write template css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "help.menu", "input.schema.json"), []byte(`{"type":"object","additionalProperties":true}`), 0o644); err != nil {
		t.Fatalf("write template input schema: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "templates", "status.panel"), 0o755); err != nil {
		t.Fatalf("mkdir status templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "status.panel", "template.html"), []byte("<html><body>{{ .title }}</body></html>"), 0o644); err != nil {
		t.Fatalf("write status template html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "status.panel", "template.json"), []byte(`{"id":"status.panel","version":"1","entry_html":"template.html","stylesheet":"styles.css","input_schema":"input.schema.json","width":960,"height":540}`), 0o644); err != nil {
		t.Fatalf("write status template json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "status.panel", "styles.css"), []byte("body { color: #111; }"), 0o644); err != nil {
		t.Fatalf("write status template css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "templates", "status.panel", "input.schema.json"), []byte(`{"type":"object","additionalProperties":true}`), 0o644); err != nil {
		t.Fatalf("write status template input schema: %v", err)
	}
}

func writePreparedRuntime(t *testing.T, repoRoot, id, version string, segments ...string) {
	t.Helper()
	target := filepath.Join(append([]string{repoRoot, ".deps", "store", id, version}, segments...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir runtime target: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write runtime target: %v", err)
	}
}

func waitTask(t *testing.T, registry *tasks.Registry, taskID string, want tasks.Status) tasks.Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok && snapshot.Status == want {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}
	snapshot, _ := registry.Get(taskID)
	t.Fatalf("task %s did not reach %s: %#v", taskID, want, snapshot)
	return tasks.Snapshot{}
}

func containsIssueCode(issues []health.DiagnosticIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func stringsNewReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
