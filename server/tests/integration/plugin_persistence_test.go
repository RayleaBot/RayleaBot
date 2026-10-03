package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestPluginDesiredStatePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-a")
	_ = issueLoginToken(t, appA)
	repositoryA, err := plugincatalog.NewSQLiteRepository(appA.Storage())
	if err != nil {
		t.Fatalf("create plugin repository: %v", err)
	}
	// Discovered plugins default to disabled, so only a persisted enabled
	// state proves the restart read the repository rather than the default.
	if err := repositoryA.SaveDesiredState(context.Background(), "raylea.echo", plugins.DesiredStateEnabled, current); err != nil {
		t.Fatalf("persist desired state: %v", err)
	}
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-b")
	defer closePersistentTestApp(t, appB)
	repositoryB, err := plugincatalog.NewSQLiteRepository(appB.Storage())
	if err != nil {
		t.Fatalf("reopen plugin repository: %v", err)
	}
	desiredStates, err := repositoryB.LoadDesiredStates(context.Background())
	if err != nil {
		t.Fatalf("load desired states: %v", err)
	}
	if desiredStates["raylea.echo"] != plugins.DesiredStateEnabled {
		t.Fatalf("persisted desired state = %q, want enabled", desiredStates["raylea.echo"])
	}
	serverB := newManagementTestServer(t, appB.Handler())
	defer serverB.Close()

	loginToken := issueExistingBootstrapLoginToken(t, appB)

	listReq, err := http.NewRequest(http.MethodGet, serverB.URL+"/api/plugins", nil)
	if err != nil {
		t.Fatalf("create plugin list request: %v", err)
	}
	listReq.Header.Set("Authorization", "Bearer "+loginToken)
	listResp, err := serverB.Client().Do(listReq)
	if err != nil {
		t.Fatalf("perform plugin list request: %v", err)
	}
	defer func(release func() error) { _ = release() }(listResp.Body.Close)
	listBody := decodeBody(t, readAll(t, listResp))
	items := listBody["items"].([]any)

	var installedEcho map[string]any
	for _, item := range items {
		entry := item.(map[string]any)
		if entry["id"] == "raylea.echo" {
			installedEcho = entry
			break
		}
	}
	if installedEcho == nil {
		t.Fatal("expected raylea.echo in plugin list")
	}
	if installedEcho["state"] != "enabled" {
		t.Fatalf("unexpected persisted state: got %#v want enabled", installedEcho["state"])
	}
}

func TestAppInitializationPreservesFailedInstallRecoveryFiles(t *testing.T) {
	var recoveryFile string
	application, _, _ := newTestAppWithOptions(t, nil, func(options *internalapp.Options, _ string) {
		recoveryFile = filepath.Join(options.PluginRoots[0].Path, ".plugin-install-recovery", "previous", "fixture.txt")
		if err := os.MkdirAll(filepath.Dir(recoveryFile), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(recoveryFile, []byte("original plugin files"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recoveryFile)
	if err != nil || string(data) != "original plugin files" {
		t.Fatalf("initialization removed failed rollback evidence: %q %v", data, err)
	}
}
