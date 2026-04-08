package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestTasksListReturnsFilteredSnapshots(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)

	taskID, err := application.Tasks.Create("plugin.install", "install weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}
	running := tasks.StatusRunning
	progress := 25
	if _, ok := application.Tasks.Update(taskID, tasks.Update{
		Status:   &running,
		Progress: &progress,
	}); !ok {
		t.Fatalf("Update task %s failed", taskID)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/tasks?status=running&task_type=plugin.install&limit=1", nil)
	if err != nil {
		t.Fatalf("create tasks list request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform tasks list request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected tasks list status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected tasks list items: %#v", body["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected task item: %#v", items[0])
	}
	if item["task_id"] != taskID {
		t.Fatalf("unexpected task_id: got %#v want %q", item["task_id"], taskID)
	}
	if item["task_type"] != "plugin.install" {
		t.Fatalf("unexpected task_type: %#v", item["task_type"])
	}
	if item["status"] != "running" {
		t.Fatalf("unexpected status: %#v", item["status"])
	}
}

func TestTaskDetailMatchesFixtureShape(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	taskID, err := application.Tasks.Create("plugin.install", "install weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.task-detail-response.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/tasks/"+taskID, nil)
	if err != nil {
		t.Fatalf("create task detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform task detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected task detail status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	taskBody := body["task"].(map[string]any)
	if taskBody["task_id"] != taskID {
		t.Fatalf("unexpected task_id: got %#v want %q", taskBody["task_id"], taskID)
	}
	if taskBody["task_type"] != fixture.Response.Body["task"].(map[string]any)["task_type"] {
		t.Fatalf("unexpected task_type: got %#v", taskBody["task_type"])
	}
	if taskBody["status"] != fixture.Response.Body["task"].(map[string]any)["status"] {
		t.Fatalf("unexpected status: got %#v", taskBody["status"])
	}
}

func TestTaskCancelAcceptsPendingTasksAndUpdatesSnapshot(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	taskID, err := application.Tasks.Create("plugin.reload", "reload weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.task-cancel-accepted.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+taskID+"/cancel", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("create task cancel request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform task cancel request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected task cancel status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	if body["task_id"] != taskID {
		t.Fatalf("unexpected task cancel body: %#v", body)
	}

	snapshot, ok := application.Tasks.Get(taskID)
	if !ok {
		t.Fatalf("expected cancelled task to remain queryable")
	}
	if snapshot.Status != tasks.StatusCancelled {
		t.Fatalf("unexpected cancelled status: got %q want %q", snapshot.Status, tasks.StatusCancelled)
	}
}

func TestTaskCancelRejectsNonCancellableState(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	taskID, err := application.Tasks.Create("plugin.reload", "reload weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}
	running := tasks.StatusRunning
	if _, ok := application.Tasks.Update(taskID, tasks.Update{Status: &running}); !ok {
		t.Fatalf("Update task %s failed", taskID)
	}

	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.tasks-cancel-not-cancellable.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+taskID+"/cancel", nil)
	if err != nil {
		t.Fatalf("create task cancel request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform task cancel request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected task cancel status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	expected := cloneMap(fixture.Response.Body)
	expectedError := cloneMap(expected["error"].(map[string]any))
	expectedDetails := cloneMap(expectedError["details"].(map[string]any))
	expectedDetails["task_id"] = taskID
	expectedError["details"] = expectedDetails
	expected["error"] = expectedError
	assertErrorEnvelopeMatchesFixture(t, body, expected, "platform.task_not_cancellable")
}

type fakeInstallCoordinator struct {
	cancelFunc func(string) bool
}

func (f fakeInstallCoordinator) Accept(_ context.Context, _ plugins.InstallRequest) (string, error) {
	return "", nil
}

func (f fakeInstallCoordinator) Cancel(taskID string) bool {
	if f.cancelFunc == nil {
		return false
	}
	return f.cancelFunc(taskID)
}

func (f fakeInstallCoordinator) Close() error {
	return nil
}

func TestTaskCancelAcceptsRunningInstallTaskViaInstaller(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	taskID, err := application.Tasks.Create("plugin.install", "install weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}
	running := tasks.StatusRunning
	if _, ok := application.Tasks.Update(taskID, tasks.Update{Status: &running}); !ok {
		t.Fatalf("Update task %s failed", taskID)
	}

	application.PluginInstaller = fakeInstallCoordinator{
		cancelFunc: func(id string) bool {
			if id != taskID {
				return false
			}
			cancelled := tasks.StatusCancelled
			now := time.Now().UTC()
			summary := "插件安装已取消"
			_, ok := application.Tasks.Update(taskID, tasks.Update{
				Status:     &cancelled,
				Summary:    &summary,
				FinishedAt: &now,
			})
			return ok
		},
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+taskID+"/cancel", nil)
	if err != nil {
		t.Fatalf("create task cancel request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform task cancel request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected task cancel status: got %d want 202", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["task_id"] != taskID {
		t.Fatalf("unexpected task cancel body: %#v", body)
	}

	snapshot, ok := application.Tasks.Get(taskID)
	if !ok {
		t.Fatalf("expected cancelled task to remain queryable")
	}
	if snapshot.Status != tasks.StatusCancelled {
		t.Fatalf("unexpected cancelled status: got %q want %q", snapshot.Status, tasks.StatusCancelled)
	}
}

func TestTaskRoutesRequireAuth(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	taskID, err := application.Tasks.Create("plugin.install", "install weather")
	if err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	paths := []string{
		"/api/tasks",
		"/api/tasks/" + taskID,
		"/api/tasks/" + taskID + "/cancel",
	}
	methods := []string{http.MethodGet, http.MethodGet, http.MethodPost}

	for i, path := range paths {
		request, err := http.NewRequest(methods[i], server.URL+path, nil)
		if err != nil {
			t.Fatalf("create request for %s: %v", path, err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("perform request for %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unexpected status for %s %s: got %d want 401", methods[i], path, response.StatusCode)
		}
	}
}

func readAll(t *testing.T, response *http.Response) []byte {
	t.Helper()

	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body.Bytes()
}

func TestTaskDetailMissingReturns404(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/tasks/task_missing_0001", nil)
	if err != nil {
		t.Fatalf("create task detail request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform task detail request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected task detail status: got %d want 404", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode task detail body: %v", err)
	}
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
}
