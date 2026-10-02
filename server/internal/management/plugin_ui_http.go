package management

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

type PluginManagementUIDeps struct {
	Plugins       plugins.CatalogView
	Settings      *settings.Service
	ActionInvoker PluginManagementActionInvoker
}

type PluginManagementUIHandlers struct {
	plugins       plugins.CatalogView
	settings      *settings.Service
	actionInvoker PluginManagementActionInvoker
}

const pluginUIPathPrefix = "/plugin-ui/"

var hashedPluginUIAssetPattern = regexp.MustCompile(`(?i)\.[0-9a-f]{8,}\.[a-z0-9]+$`)

type PluginManagementActionInvoker interface {
	InvokeManagementAction(context.Context, string, string, map[string]any) (map[string]any, error)
}

type pluginManagementActionRequest struct {
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload,omitempty"`
}

type PluginManagementActionResponse struct {
	PluginID string         `json:"plugin_id"`
	Action   string         `json:"action"`
	Result   map[string]any `json:"result"`
}

func NewPluginManagementUIHandlers(deps PluginManagementUIDeps) *PluginManagementUIHandlers {
	return &PluginManagementUIHandlers{
		plugins:       deps.Plugins,
		settings:      deps.Settings,
		actionInvoker: deps.ActionInvoker,
	}
}

func (h *PluginManagementUIHandlers) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get(pluginUIPathPrefix+"{plugin_id}/*", h.HandlePluginUIAsset())
	router.Head(pluginUIPathPrefix+"{plugin_id}/*", h.HandlePluginUIAsset())
}

func (h *PluginManagementUIHandlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsGet())
	router.Put("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsPut())
	router.Get("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsPut())
	router.Delete("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsDelete())
	router.Post("/api/plugins/{plugin_id}/management/actions", h.HandlePluginManagementAction())
}

func (h *PluginManagementUIHandlers) HandlePluginManagementAction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
		actionInvoker := h.actionInvoker
		if pluginID == "" {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		if _, ok := h.resolveSettingsSnapshot(w, r); !ok {
			return
		}
		if actionInvoker == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}

		var request pluginManagementActionRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		action := strings.TrimSpace(request.Action)
		if action == "" {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		if request.Payload == nil {
			request.Payload = map[string]any{}
		}

		result, err := actionInvoker.InvokeManagementAction(r.Context(), pluginID, action, request.Payload)
		if err != nil {
			var unavailable *plugins.NotRunningError
			if errors.As(err, &unavailable) {
				httpapi.WriteError(w, r, errorcodes.PluginNotRunning, unavailable.Details())
				return
			}
			httpapi.WriteDomainError(w, r, &httpapi.DomainError{
				Code: errorcodes.PluginManagementActionFailed,
			})
			return
		}
		if result == nil {
			result = map[string]any{}
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginManagementActionResponse{
			PluginID: pluginID,
			Action:   action,
			Result:   result,
		})
	}
}

func (h *PluginManagementUIHandlers) HandlePluginUIAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.plugins.Get(strings.TrimSpace(chi.URLParam(r, "plugin_id")))
		if !ok || !pluginUISnapshotReady(snapshot) {
			http.NotFound(w, r)
			return
		}
		h.servePluginUIAsset(w, r, snapshot)
	}
}

func (h *PluginManagementUIHandlers) servePluginUIAsset(w http.ResponseWriter, r *http.Request, snapshot plugins.Snapshot) {

	assetRoot := pluginUIAssetRoot(snapshot)
	if assetRoot == "" {
		http.NotFound(w, r)
		return
	}
	assetPath := normalizePluginUIAssetPath(chi.URLParam(r, "*"))
	if assetPath == "" {
		assetPath = strings.TrimPrefix(strings.TrimSpace(snapshot.ManagementUI.Entry), "ui/")
	}
	assetFile := filepath.Clean(filepath.Join(assetRoot, filepath.FromSlash(assetPath)))
	if !fsguard.WithinRoot(assetRoot, assetFile) {
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

	writePluginUIHeaders(w, r, snapshot.PluginID, assetPath)
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func pluginUISnapshotReady(snapshot plugins.Snapshot) bool {
	return snapshot.Valid && snapshot.RegistrationState == "installed" && snapshot.ArtifactVersion == artifact.Version &&
		snapshot.ArtifactUIAvailable && snapshot.ManagementUI != nil && strings.TrimSpace(snapshot.PackageRootPath) != "" &&
		len(snapshot.ManagementUI.Pages) > 0 && strings.HasPrefix(strings.TrimSpace(snapshot.ManagementUI.Entry), "ui/")
}

func (h *PluginManagementUIHandlers) resolveSettingsSnapshot(w http.ResponseWriter, r *http.Request) (plugins.Snapshot, bool) {
	if h.plugins == nil {
		httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
		return plugins.Snapshot{}, false
	}

	pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
	snapshot, ok := h.plugins.Get(pluginID)
	if !ok {
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
			"resource_type": "plugin",
			"plugin_id":     pluginID,
		})
		return plugins.Snapshot{}, false
	}

	if !snapshot.Valid {
		details := map[string]any{
			"plugin_id": pluginID,
		}
		if snapshot.DisplayState == "conflict" {
			details["kind"] = "plugin_id_conflict"
			details["manifest_paths"] = append([]string(nil), snapshot.ConflictPaths...)
			details["source_roots"] = append([]string(nil), snapshot.SourceRoots...)
		} else {
			details["kind"] = "invalid_manifest"
			details["manifest_path"] = snapshot.ManifestPath
			details["validation_summary"] = snapshot.ValidationSummary
		}
		httpapi.WriteError(w, r, errorcodes.PlatformStateConflict, details)
		return plugins.Snapshot{}, false
	}

	if snapshot.RegistrationState != "installed" {
		httpapi.WriteError(w, r, errorcodes.PlatformStateConflict, map[string]any{
			"plugin_id": pluginID,
			"kind":      "plugin_not_installed",
			"installed": false,
		})
		return plugins.Snapshot{}, false
	}

	return snapshot, true
}

func pluginUIAssetRoot(snapshot plugins.Snapshot) string {
	if snapshot.ManagementUI == nil || strings.TrimSpace(snapshot.PackageRootPath) == "" {
		return ""
	}

	if len(snapshot.ManagementUI.Pages) == 0 || !strings.HasPrefix(strings.TrimSpace(snapshot.ManagementUI.Entry), "ui/") {
		return ""
	}
	return filepath.Clean(filepath.Join(snapshot.PackageRootPath, "ui"))
}

func normalizePluginUIAssetPath(assetPath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(assetPath))
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func writePluginUIHeaders(w http.ResponseWriter, r *http.Request, pluginID, assetPath string) {
	header := w.Header()
	if hashedPluginUIAssetPattern.MatchString(path.Base(assetPath)) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-store, max-age=0")
	}
	header.Set("Content-Security-Policy", pluginUIContentSecurityPolicy(r.Host, pluginID))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")
}

// pluginUIContentSecurityPolicy limits scripts to the plugin UI path. A
// host-source without a scheme follows the page scheme; CSP cannot express an
// IPv6 literal host, which falls back to 'self'. Path matching only holds
// without redirects, so the asset route never redirects. Images and media may
// come from any HTTPS origin because they cannot run code; Referrer-Policy
// keeps the management path out of those requests.
func pluginUIContentSecurityPolicy(host, pluginID string) string {
	scriptSource := "'self'"
	if host = strings.TrimSpace(host); host != "" && !strings.HasPrefix(host, "[") {
		scriptSource = host + pluginUIPathPrefix + pluginID + "/"
	}
	return "default-src 'none'; script-src " + scriptSource + "; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; media-src 'self' data: https:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"
}
