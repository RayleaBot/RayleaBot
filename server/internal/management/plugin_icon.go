package management

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const pluginIconMaxBytes = 512 * 1024

func newPluginIconHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, exists := catalog.Get(chi.URLParam(r, "plugin_id"))
		if !exists || !snapshot.Valid || snapshot.RegistrationState != "installed" {
			writePluginIconMissing(w, r)
			return
		}
		body, contentType := readPluginIcon(snapshot)
		if contentType == "" {
			writePluginIconMissing(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

func writePluginIconMissing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	writeError(w, r, http.StatusNotFound, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", nil)
}

func readPluginIcon(snapshot plugins.Snapshot) ([]byte, string) {
	name := strings.TrimSpace(snapshot.Icon)
	if snapshot.PackageRootPath == "" || name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
		return nil, ""
	}
	root, err := os.OpenRoot(snapshot.PackageRootPath)
	if err != nil {
		return nil, ""
	}
	defer func(release func() error) { _ = release() }(root.Close)
	info, err := root.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > pluginIconMaxBytes {
		return nil, ""
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, ""
	}
	defer func(release func() error) { _ = release() }(file.Close)
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > pluginIconMaxBytes {
		return nil, ""
	}
	body, err := io.ReadAll(io.LimitReader(file, pluginIconMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > pluginIconMaxBytes {
		return nil, ""
	}
	return body, pluginIconContentType(body)
}

func pluginIconContentType(body []byte) string {
	contentType := http.DetectContentType(body)
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return contentType
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	rootSeen := false
	for {
		token, err := decoder.Token()
		if err == io.EOF && rootSeen {
			return "image/svg+xml"
		}
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case xml.Directive:
			return ""
		case xml.StartElement:
			if !rootSeen {
				if value.Name.Local != "svg" || (value.Name.Space != "" && value.Name.Space != "http://www.w3.org/2000/svg") {
					return ""
				}
				rootSeen = true
			}
		}
	}
}
