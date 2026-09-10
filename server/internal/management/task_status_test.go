package management

import (
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskStatusReturnsLifecycleWithoutPrivateTaskDetails(t *testing.T) {
	handlers, registry := newTaskOnlyHandlers(t, t.TempDir())
	id, err := registry.Create("plugin.install", "private-summary")
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	handlers.RegisterProtectedRoutes(router, nil)
	for _, state := range []tasks.Status{tasks.StatusPending, tasks.StatusRunning, tasks.StatusSucceeded, tasks.StatusFailed, tasks.StatusCancelled, tasks.StatusInterrupted} {
		registry.Update(id, tasks.Update{Status: &state, Error: &tasks.ErrorSummary{Code: "plugin.internal_error", Message: "private-error", Details: map[string]any{"fixture_secret": "fixture-only"}}})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/tasks/"+id, nil))
		if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("status response: %d %s", response.Code, response.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["task_id"] != id || body["status"] != string(state) || body["error_code"] != "plugin.internal_error" || len(body) != 3 {
			t.Fatalf("task status: %#v", body)
		}
		if strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), "fixture-only") {
			t.Fatal("private task data exposed")
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/tasks/missing", nil))
	if response.Code != 404 || !strings.Contains(response.Body.String(), "platform.resource_not_found") {
		t.Fatalf("missing task: %d %s", response.Code, response.Body.String())
	}
}
