package ui

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func NewManagementUIHandler(repoRoot string) http.HandlerFunc {
	distRoot := filepath.Join(repoRoot, "web", "dist")
	indexPath := filepath.Join(distRoot, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/ws/") ||
			r.URL.Path == "/healthz" ||
			r.URL.Path == "/readyz" {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat(indexPath); err != nil {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean("/" + strings.TrimSpace(r.URL.Path))
		if cleanPath == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}

		assetPath, ok := staticAssetPath(distRoot, r.URL.Path)
		if !ok {
			http.ServeFile(w, r, indexPath)
			return
		}
		if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, assetPath)
			return
		}

		http.ServeFile(w, r, indexPath)
	}
}

func staticAssetPath(distRoot string, requestPath string) (string, bool) {
	normalizedPath := strings.ReplaceAll(strings.TrimSpace(requestPath), "\\", "/")
	relativePath := path.Clean(strings.TrimPrefix(normalizedPath, "/"))
	if relativePath == "." || relativePath == ".." || path.IsAbs(relativePath) || strings.HasPrefix(relativePath, "../") {
		return "", false
	}
	targetPath := filepath.Join(distRoot, filepath.FromSlash(relativePath))
	if !pathWithinRoot(distRoot, targetPath) {
		return "", false
	}
	return targetPath, true
}

func pathWithinRoot(root, candidate string) bool {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteCandidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
