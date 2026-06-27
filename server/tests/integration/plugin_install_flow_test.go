package integration

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

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
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
		PluginRoots: []plugindiscovery.ScanRoot{
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

	taskSnapshot := waitForTaskStatus(t, application.Tasks(), taskID, tasks.StatusSucceeded)
	if taskSnapshot.TaskType != "plugin.install" {
		t.Fatalf("unexpected task_type: got %q want %q", taskSnapshot.TaskType, "plugin.install")
	}
	if taskSnapshot.Result == nil || taskSnapshot.Result.Summary == "" {
		t.Fatalf("expected task result summary, got %#v", taskSnapshot.Result)
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
	if plugin["state"] != "disabled" {
		t.Fatalf("unexpected state: got %#v want %q", plugin["state"], "disabled")
	}

	if _, err := os.Stat(filepath.Join(installedRoot, "weather-install", "info.json")); err != nil {
		t.Fatalf("expected installed manifest to exist: %v", err)
	}

	var (
		sourceType   string
		sourceRef    string
		version      string
		manifestHash string
		packageHash  string
	)
	if err := application.Storage().Read.QueryRow(
		`SELECT source_type, source_ref, version, manifest_hash, package_hash
		   FROM plugin_packages
		  WHERE plugin_id = ?`,
		"weather-install",
	).Scan(&sourceType, &sourceRef, &version, &manifestHash, &packageHash); err != nil {
		t.Fatalf("query plugin_packages row: %v", err)
	}
	if sourceType != "local_directory" {
		t.Fatalf("unexpected source_type metadata: got %q want local_directory", sourceType)
	}
	if sourceRef != sourceDir {
		t.Fatalf("unexpected source_ref metadata: got %q want %q", sourceRef, sourceDir)
	}
	if version != "0.1.0" || manifestHash == "" || packageHash == "" {
		t.Fatalf("unexpected package metadata values: version=%q manifest_hash=%q package_hash=%q", version, manifestHash, packageHash)
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

	deadline := time.Now().Add(taskStatusTimeout())
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

func taskStatusTimeout() time.Duration {
	if testing.CoverMode() != "" || testRaceEnabled {
		return 20 * time.Second
	}
	return 15 * time.Second
}
