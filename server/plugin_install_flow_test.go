package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rayleabot/server/internal/app"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/tasks"
)

func TestPluginInstallRouteExecutesTaskAndRefreshesCatalog(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	pluginSchemaPath := filepath.Join("..", "contracts", "plugin-info.schema.json")
	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")

	for _, root := range []string{examplesRoot, installedRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("create plugin discovery root %s: %v", root, err)
		}
	}

	sessionCounter := 0
	application, err := app.New(app.Options{
		ConfigPath:       configPath,
		SchemaPath:       filepath.Join("..", "contracts", "config.user.schema.json"),
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: pluginSchemaPath,
		PluginRoots: []plugins.ScanRoot{
			{Label: "examples/plugins", Path: examplesRoot},
			{Label: "plugins/installed", Path: installedRoot},
		},
		AuthOptions: []auth.Option{
			auth.WithClock(func() time.Time {
				return time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
			}),
			auth.WithSessionIDGenerator(func() (string, error) {
				sessionCounter++
				return fmt.Sprintf("install-flow-%d", sessionCounter), nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	token := issueLoginToken(t, application)
	sourceDir := writePluginInstallSource(t, filepath.Join(t.TempDir(), "weather-source"), "weather-install", "nodejs", "index.js")

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	requestBody, err := json.Marshal(map[string]any{
		"source_type": "local_directory",
		"source":      sourceDir,
	})
	if err != nil {
		t.Fatalf("marshal install request: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/plugins/install", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("create install request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform install request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected install status: got %d want 202", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("unexpected install response body: %#v", body)
	}

	taskSnapshot := waitForTaskStatus(t, application.Tasks, taskID, tasks.StatusSucceeded)
	if taskSnapshot.TaskType != "plugin.install" {
		t.Fatalf("unexpected task_type: got %q want %q", taskSnapshot.TaskType, "plugin.install")
	}
	if taskSnapshot.Result == nil || taskSnapshot.Result.Summary == "" {
		t.Fatalf("expected task result summary, got %#v", taskSnapshot.Result)
	}

	taskRequest, err := http.NewRequest(http.MethodGet, server.URL+"/api/tasks/"+taskID, nil)
	if err != nil {
		t.Fatalf("create task detail request: %v", err)
	}
	taskRequest.Header.Set("Authorization", "Bearer "+token)

	taskResponse, err := server.Client().Do(taskRequest)
	if err != nil {
		t.Fatalf("perform task detail request: %v", err)
	}
	defer taskResponse.Body.Close()
	if taskResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected task detail status: got %d want 200", taskResponse.StatusCode)
	}

	taskBody := decodeBody(t, readAll(t, taskResponse))
	task := taskBody["task"].(map[string]any)
	if task["task_id"] != taskID {
		t.Fatalf("unexpected task detail id: got %#v want %q", task["task_id"], taskID)
	}
	if task["status"] != string(tasks.StatusSucceeded) {
		t.Fatalf("unexpected task detail status: got %#v want %q", task["status"], tasks.StatusSucceeded)
	}

	pluginRequest, err := http.NewRequest(http.MethodGet, server.URL+"/api/plugins/weather-install", nil)
	if err != nil {
		t.Fatalf("create plugin detail request: %v", err)
	}
	pluginRequest.Header.Set("Authorization", "Bearer "+token)

	pluginResponse, err := server.Client().Do(pluginRequest)
	if err != nil {
		t.Fatalf("perform plugin detail request: %v", err)
	}
	defer pluginResponse.Body.Close()
	if pluginResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected plugin detail status: got %d want 200", pluginResponse.StatusCode)
	}

	pluginBody := decodeBody(t, readAll(t, pluginResponse))
	plugin := pluginBody["plugin"].(map[string]any)
	if plugin["id"] != "weather-install" {
		t.Fatalf("unexpected plugin id: got %#v want %q", plugin["id"], "weather-install")
	}
	if plugin["registration_state"] != "installed" {
		t.Fatalf("unexpected registration_state: got %#v want %q", plugin["registration_state"], "installed")
	}
	if plugin["desired_state"] != "disabled" {
		t.Fatalf("unexpected desired_state: got %#v want %q", plugin["desired_state"], "disabled")
	}

	if _, err := os.Stat(filepath.Join(installedRoot, "weather-install", "info.json")); err != nil {
		t.Fatalf("expected installed manifest to exist: %v", err)
	}
}

func writePluginInstallSource(t *testing.T, root, pluginID, runtimeName, entry string) string {
	t.Helper()

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create plugin root: %v", err)
	}

	manifest := map[string]any{
		"id":                      pluginID,
		"name":                    pluginID,
		"version":                 "0.1.0",
		"manifest_version":        "1",
		"plugin_protocol_version": "1",
		"type":                    "managed_runtime",
		"runtime":                 runtimeName,
		"entry":                   entry,
		"license":                 "MIT",
		"description":             "test plugin",
		"author":                  "raylea",
		"capabilities":            []string{"event.subscribe"},
		"permissions": map[string]any{
			"required": []string{},
			"optional": []string{},
		},
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	entryContent := "console.log('ok')\n"
	if runtimeName == "python" {
		entryContent = "print('ok')\n"
	}
	if err := os.WriteFile(filepath.Join(root, entry), []byte(entryContent), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	return root
}

func waitForTaskStatus(t *testing.T, registry *tasks.Registry, taskID string, want tasks.Status) tasks.Snapshot {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok {
			if snapshot.Status == want {
				return snapshot
			}
			switch snapshot.Status {
			case tasks.StatusSucceeded, tasks.StatusFailed, tasks.StatusCancelled, tasks.StatusInterrupted:
				t.Fatalf("task %s reached terminal status %q before %q: %#v", taskID, snapshot.Status, want, snapshot)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot, _ := registry.Get(taskID)
	t.Fatalf("timed out waiting for task %s to reach %q; last snapshot=%#v", taskID, want, snapshot)
	return tasks.Snapshot{}
}
