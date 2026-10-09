package management

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestPluginUIAssetRejectsSymlinkOutsideUI(t *testing.T) {
	packageRoot := t.TempDir()
	uiRoot := filepath.Join(packageRoot, "ui")
	if err := os.Mkdir(uiRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(packageRoot, "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(uiRoot, "leak.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	snapshot := plugins.Snapshot{PluginID: "test", PackageRootPath: packageRoot, ManagementUI: &plugins.ManagementUI{Entry: "ui/index.html", Pages: []plugins.ManagementUIPage{{ID: "main"}}}}
	router := chi.NewRouter()
	handler := &PluginManagementUIHandlers{}
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) { handler.servePluginUIAsset(w, r, snapshot) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/leak.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlink status=%d body=%q", response.Code, response.Body.String())
	}
}
