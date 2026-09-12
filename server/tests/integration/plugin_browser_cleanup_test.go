package integration

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestUninstallRemovesOnlyTheStoppedPluginsBrowserProfiles(t *testing.T) {
	t.Parallel()
	var profileRoot string
	application, _, _ := newTestAppWithOptions(t, nil, func(options *app.Options, _ string) {
		profileRoot = filepath.Join(options.PluginRepoRoot, "data", "plugin-browser")
		for _, pluginID := range []string{"retired-plugin", "other-plugin"} {
			if err := os.MkdirAll(filepath.Join(profileRoot, pluginID, "login"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	})
	token := issueLoginToken(t, application)
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Host = testutil.TestManagementAuthority
		req.RemoteAddr = "127.0.0.1:0"
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		application.Handler().ServeHTTP(response, req)
		return response
	}
	response := request(http.MethodDelete, "/api/plugins/retired-plugin")
	if response.Code != http.StatusAccepted {
		t.Fatalf("uninstall = %d %s", response.Code, response.Body.String())
	}
	taskID, ok := decodeBody(t, response.Body.Bytes())["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatal("missing uninstall task")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response = request(http.MethodGet, "/api/system/tasks/"+taskID)
		if response.Code != http.StatusOK {
			t.Fatalf("task = %d %s", response.Code, response.Body.String())
		}
		task := decodeBody(t, response.Body.Bytes())
		if task["status"] == "succeeded" {
			break
		}
		if task["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("uninstall did not finish: %#v", task)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(profileRoot, "retired-plugin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uninstalled profile remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profileRoot, "other-plugin", "login")); err != nil {
		t.Fatalf("other profile changed: %v", err)
	}
}
