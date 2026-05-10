package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestPluginDesiredStatePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))

	appA := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-a")
	token := issueLoginToken(t, appA)
	serverA := httptest.NewServer(appA.Handler())

	enableReq, err := http.NewRequest(http.MethodPost, serverA.URL+"/api/plugins/raylea.echo/disable", nil)
	if err != nil {
		t.Fatalf("create disable request: %v", err)
	}
	enableReq.Header.Set("Authorization", "Bearer "+token)
	enableResp, err := serverA.Client().Do(enableReq)
	if err != nil {
		t.Fatalf("perform disable request: %v", err)
	}
	enableResp.Body.Close()
	if enableResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected disable status: got %d want 200", enableResp.StatusCode)
	}
	serverA.Close()
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return current }, "plugin-b")
	defer closePersistentTestApp(t, appB)
	serverB := httptest.NewServer(appB.Handler())
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

	var builtinEcho map[string]any
	for _, item := range items {
		entry := item.(map[string]any)
		if entry["id"] == "raylea.echo" {
			builtinEcho = entry
			break
		}
	}
	if builtinEcho == nil {
		t.Fatal("expected raylea.echo in plugin list")
	}
	if builtinEcho["desired_state"] != "disabled" {
		t.Fatalf("unexpected persisted desired_state: got %#v want disabled", builtinEcho["desired_state"])
	}
	if builtinEcho["runtime_state"] != "stopped" {
		t.Fatalf("unexpected runtime_state after restart: got %#v want stopped", builtinEcho["runtime_state"])
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
