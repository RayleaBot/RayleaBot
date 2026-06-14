package managementui

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func (h *Handlers) HandlePluginManagementUIStatic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		snapshot, ok := h.resolvePluginUISnapshot(chi.URLParam(r, "plugin_id"))
		if !ok {
			http.NotFound(w, r)
			return
		}

		assetPath := normalizePluginUIAssetPath(chi.URLParam(r, "*"))
		if assetPath == "" {
			http.NotFound(w, r)
			return
		}

		assetRoot := pluginUIAssetRoot(snapshot)
		if assetRoot == "" {
			http.NotFound(w, r)
			return
		}

		assetFile := filepath.Clean(filepath.Join(snapshot.PackageRootPath, filepath.FromSlash(assetPath)))
		if !isPathWithinRoot(assetRoot, assetFile) {
			http.NotFound(w, r)
			return
		}

		file, err := os.Open(assetFile)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = file.Close() }()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		writeNoStoreHeaders(w)
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}
}

func writeNoStoreHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Cache-Control", "no-store, max-age=0")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}
