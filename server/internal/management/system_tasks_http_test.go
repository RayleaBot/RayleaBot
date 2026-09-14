package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
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
	service, err := system.New(system.Deps{
		CurrentConfig:    func() config.Config { return config.Config{} },
		CurrentSummary:   func() config.Summary { return config.Summary{} },
		CurrentRepoRoot:  func() string { return repoRoot },
		CurrentStartedAt: func() time.Time { return startedAt },
		Plugins:          plugincatalog.New(nil),
		TaskExecutor:     executor,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewSystemHandlers(service), registry
}

func TestHandleSystemRuntimeBootstrapAcceptsTaskAndReportsPreparedStoreHits(t *testing.T) {
	repoRoot := t.TempDir()
	testutil.WritePlatformDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	testutil.WritePreparedRuntime(t, repoRoot, "chromium-"+platform, "152.0.7977.42", "chrome-win64", "chrome.exe")

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
