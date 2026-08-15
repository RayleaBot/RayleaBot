package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func newTaskOnlyHandlers(t *testing.T, repoRoot string) (*SystemHandlers, *tasks.Registry) {
	t.Helper()
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 2*time.Second)
	t.Cleanup(func() {
		_ = executor.Close()
	})
	startedAt := time.Now()
	service := system.New(system.Deps{
		CurrentConfig:    func() config.Config { return config.Config{} },
		CurrentSummary:   func() config.Summary { return config.Summary{} },
		CurrentRepoRoot:  func() string { return repoRoot },
		CurrentStartedAt: func() time.Time { return startedAt },
		Plugins:          plugincatalog.New(nil),
		TaskExecutor:     executor,
	})
	return NewSystemHandlers(service), registry
}

func TestSystemTaskQueueFullMapsToTooManyRequests(t *testing.T) {
	httpErr := systemHTTPErrorFromError(system.TaskQueueFullError())
	if httpErr == nil || httpErr.statusCode != http.StatusTooManyRequests || httpErr.code != "platform.task_queue_full" {
		t.Fatalf("unexpected task queue mapping: %#v", httpErr)
	}
}

func TestHandleSystemRecoveryRecheckAcceptsTaskAndPersistsCompatibleSummary(t *testing.T) {
	repoRoot := t.TempDir()
	testutil.WritePlatformDepsManifest(t, repoRoot)
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

	handlers, registry := newTaskOnlyHandlers(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/recheck", nil)
	recorder := httptest.NewRecorder()

	handlers.HandleSystemRecoveryRecheck().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := testutil.WaitTask(t, registry, accepted.TaskID, tasks.StatusSucceeded)
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
	handlers, _ := newTaskOnlyHandlers(t, t.TempDir())
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/recheck", nil)
	recorder := httptest.NewRecorder()

	handlers.HandleSystemRecoveryRecheck().ServeHTTP(recorder, request)

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

	handlers, registry := newTaskOnlyHandlers(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/confirm", strings.NewReader(`{"review_ids":["`+initial.SkippedPlugins[0].ReviewID+`"],"note":"已确认当前跳过状态。"}`))
	request = request.WithContext(ContextWithClaims(request.Context(), auth.Claims{Subject: "alice"}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.HandleSystemRecoveryConfirm().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202 (%s)", recorder.Code, recorder.Body.String())
	}

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := testutil.WaitTask(t, registry, accepted.TaskID, tasks.StatusSucceeded)
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

	handlers, _ := newTaskOnlyHandlers(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/recovery/confirm", strings.NewReader(`{"review_ids":["review_missing"]}`))
	request = request.WithContext(ContextWithClaims(request.Context(), auth.Claims{Subject: "alice"}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.HandleSystemRecoveryConfirm().ServeHTTP(recorder, request)

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
	testutil.WritePlatformDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	testutil.WritePreparedRuntime(t, repoRoot, "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe")

	handlers, registry := newTaskOnlyHandlers(t, repoRoot)
	request := httptest.NewRequest(http.MethodPost, "/api/system/runtime/bootstrap", strings.NewReader(`{"resources":["chromium"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handlers.HandleSystemRuntimeBootstrap().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}
	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode task accepted response: %v", err)
	}
	snapshot := testutil.WaitTask(t, registry, accepted.TaskID, tasks.StatusSucceeded)
	if snapshot.TaskType != "runtime.bootstrap" {
		t.Fatalf("unexpected task type: %#v", snapshot)
	}
	if snapshot.Result == nil {
		t.Fatalf("expected task result, got %#v", snapshot)
	}
	resources, ok := snapshot.Result.Details["resources"].([]any)
	if !ok || len(resources) != 1 {
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
	if _, ok := first["used_system_browser"]; !ok {
		t.Fatalf("runtime bootstrap result should expose used_system_browser: %#v", first)
	}
}

func TestHandleSystemRuntimeBootstrapRejectsRetiredPluginRuntimes(t *testing.T) {
	handlers, _ := newTaskOnlyHandlers(t, t.TempDir())
	request := httptest.NewRequest(http.MethodPost, "/api/system/runtime/bootstrap", strings.NewReader(`{"resources":["python-runtime"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handlers.HandleSystemRuntimeBootstrap().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
