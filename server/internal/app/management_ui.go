package app

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func newManagementUIHandler(repoRoot string) http.HandlerFunc {
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

		assetPath := filepath.Join(distRoot, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
		if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, assetPath)
			return
		}

		http.ServeFile(w, r, indexPath)
	}
}
