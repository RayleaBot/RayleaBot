package integration

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestPluginDesiredStatePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-a")
	_ = issueLoginToken(t, appA)
	repositoryA, err := plugins.NewSQLiteRepository(appA.Storage())
	if err != nil {
		t.Fatalf("create plugin repository: %v", err)
	}
	if err := repositoryA.SaveDesiredState(context.Background(), "raylea.echo", plugins.DesiredStateDisabled, current); err != nil {
		t.Fatalf("persist desired state: %v", err)
	}
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-b")
	defer closePersistentTestApp(t, appB)
	repositoryB, err := plugins.NewSQLiteRepository(appB.Storage())
	if err != nil {
		t.Fatalf("reopen plugin repository: %v", err)
	}
	desiredStates, err := repositoryB.LoadDesiredStates(context.Background())
	if err != nil {
		t.Fatalf("load desired states: %v", err)
	}
	if desiredStates["raylea.echo"] != plugins.DesiredStateDisabled {
		t.Fatalf("persisted desired state = %q, want disabled", desiredStates["raylea.echo"])
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
	defer listResp.Body.Close()
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
	if installedEcho["state"] != "disabled" {
		t.Fatalf("unexpected persisted state: got %#v want disabled", installedEcho["state"])
	}
}

func issueExistingBootstrapLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	t.Helper()

	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))
	login := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	body := decodeBody(t, login.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}

	return token
}
