package integration

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestDevelopmentSyncStartsOnlyTargetAndRollsBackFailedInitialization(t *testing.T) {
	root, artifacts := t.TempDir(), t.TempDir()
	installed := filepath.Join(root, "plugins", "installed")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(app.Options{
		ConfigPath: writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db")),
		SchemaPath: testutil.RepoPath(t, "contracts", "config.user.schema.json"),
		SetupToken: testutil.TestSetupToken, LauncherControlToken: testutil.TestLauncherControlToken,
		PluginRepoRoot: root, PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots:             []plugincatalog.ScanRoot{{Label: "plugins/installed", Path: installed}},
		DevelopmentArtifactRoot: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Error(err)
		}
	})
	sync := func(artifact string, want tasks.Status) bool {
		t.Helper()
		body, err := json.Marshal(map[string]string{"artifact": artifact, "source": root})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", "/api/development/plugins/sync", bytes.NewReader(body))
		request.Host = testManagementAuthority
		request.RemoteAddr = "127.0.0.1:1234"
		request.Header.Set("X-Raylea-Launcher-Control", testutil.TestLauncherControlToken)
		response := httptest.NewRecorder()
		application.Handler().ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("sync HTTP %d: %s", response.Code, response.Body.String())
		}
		var result struct {
			Changed bool   `json:"changed"`
			TaskID  string `json:"task_id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Changed {
			waitForTaskStatus(t, application.Tasks(), result.TaskID, want)
		}
		return result.Changed
	}
	first := testutil.WriteGoPluginArtifact(t, filepath.Join(artifacts, "first"), "development.first", "0.4.0")
	second := testutil.WriteGoPluginArtifact(t, filepath.Join(artifacts, "second"), "development.second", "0.4.0")
	sync(first, tasks.StatusSucceeded)
	sync(second, tasks.StatusSucceeded)
	for _, id := range []string{"development.first", "development.second"} {
		snapshot, exists := application.Plugins().Get(id)
		if !exists || snapshot.RuntimeState != "running" {
			t.Fatalf("plugin %s state=%#v", id, snapshot)
		}
	}
	if sync(first, tasks.StatusSucceeded) {
		t.Fatal("unchanged package was reinstalled")
	}
	bad := testutil.WriteGoPluginArtifact(t, filepath.Join(artifacts, "bad"), "development.first", "0.4.1")
	program := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(program, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := "plugin"
	if runtime.GOOS == "windows" {
		entry += ".exe"
	}
	command := exec.Command("go", "build", "-o", filepath.Join(bad, "bin", entry), program)
	command.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build failing fixture: %v %s", err, output)
	}
	sync(bad, tasks.StatusFailed)
	for _, id := range []string{"development.first", "development.second"} {
		snapshot, _ := application.Plugins().Get(id)
		if snapshot.Version != "0.4.0" || snapshot.RuntimeState != "running" {
			t.Fatalf("rollback state=%#v", snapshot)
		}
	}
}
