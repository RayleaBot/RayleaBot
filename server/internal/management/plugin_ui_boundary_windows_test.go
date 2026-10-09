//go:build windows

package management

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestPluginUIAssetRejectsWindowsJunctionOutsideUI(t *testing.T) {
	packageRoot, outside := t.TempDir(), t.TempDir()
	uiRoot := filepath.Join(packageRoot, "ui")
	if err := os.Mkdir(uiRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(uiRoot, "linked"), outside)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v %s", err, output)
	}
	snapshot := plugins.Snapshot{PluginID: "test", PackageRootPath: packageRoot, ManagementUI: &plugins.ManagementUI{Entry: "ui/index.html", Pages: []plugins.ManagementUIPage{{ID: "main"}}}}
	handler := &PluginManagementUIHandlers{}
	router := chi.NewRouter()
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) { handler.servePluginUIAsset(w, r, snapshot) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/linked/private.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("junction status=%d body=%q", response.Code, response.Body.String())
	}
}
