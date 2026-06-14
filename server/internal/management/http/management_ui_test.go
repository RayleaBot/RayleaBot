package managementhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestManagementUIHandlerServesIndexForSpaRoutes(t *testing.T) {
	repoRoot := t.TempDir()
	distRoot := filepath.Join(repoRoot, "web", "dist")
	if err := os.MkdirAll(filepath.Join(distRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir dist root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distRoot, "index.html"), []byte("<html>launcher ui</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distRoot, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	handler := NewManagementUIHandler(repoRoot)

	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexRec := httptest.NewRecorder()
	handler(indexRec, indexReq)
	if indexRec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", indexRec.Code, http.StatusOK)
	}

	setupReq := httptest.NewRequest(http.MethodGet, "/setup", nil)
	setupRec := httptest.NewRecorder()
	handler(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want %d", setupRec.Code, http.StatusOK)
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetRec := httptest.NewRecorder()
	handler(assetRec, assetReq)
	if assetRec.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d", assetRec.Code, http.StatusOK)
	}
	if got := assetRec.Body.String(); got != "console.log('ok')" {
		t.Fatalf("asset body = %q, want asset content", got)
	}
}

func TestManagementUIHandlerKeepsApiPathsNotFound(t *testing.T) {
	handler := NewManagementUIHandler(t.TempDir())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/not-found", nil)
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
