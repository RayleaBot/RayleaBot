package management

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type PluginManagementUIDeps struct {
	Plugins            plugins.CatalogView
	PluginConfig       pluginstore.ConfigRepository
	Secrets            secrets.Store
	NotifyConfigChange func(context.Context, string)
	RefreshCommands    func(context.Context, string, map[string]any)
	ActionInvoker      PluginManagementActionInvoker
}

type PluginManagementUIHandlers struct {
	plugins            plugins.CatalogView
	pluginConfig       pluginstore.ConfigRepository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
	actionInvoker      PluginManagementActionInvoker
}

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
		plugins:            deps.Plugins,
		pluginConfig:       deps.PluginConfig,
		secrets:            deps.Secrets,
		notifyConfigChange: deps.NotifyConfigChange,
		refreshCommands:    deps.RefreshCommands,
		actionInvoker:      deps.ActionInvoker,
	}
}

func (h *PluginManagementUIHandlers) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
	router.Head("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
}

func (h *PluginManagementUIHandlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsGet())
	router.Put("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsPut())
	router.Get("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsPut())
	router.Post("/api/plugins/{plugin_id}/management/actions", h.HandlePluginManagementAction())
}

func (h *PluginManagementUIHandlers) HandlePluginManagementAction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
		actionInvoker := h.actionInvoker
		if pluginID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if _, ok := h.resolveSettingsSnapshot(w, r); !ok {
			return
		}
		if actionInvoker == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var request pluginManagementActionRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		action := strings.TrimSpace(request.Action)
		if action == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if request.Payload == nil {
			request.Payload = map[string]any{}
		}

		result, err := actionInvoker.InvokeManagementAction(r.Context(), pluginID, action, request.Payload)
		if err != nil {
			httpapi.WriteDomainError(w, r, &httpapi.DomainError{
				Code:        "plugin.internal_error",
				HTTPStatus:  http.StatusBadGateway,
				SafeMessage: "插件操作失败",
				MessageKey:  "errors.plugin.internal_error",
				Cause:       err,
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

func (h *PluginManagementUIHandlers) HandlePluginManagementUIStatic() http.HandlerFunc {
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

func (h *PluginManagementUIHandlers) resolvePluginUISnapshot(pluginID string) (plugins.Snapshot, bool) {
	if h.plugins == nil {
		return plugins.Snapshot{}, false
	}

	snapshot, ok := h.plugins.Get(strings.TrimSpace(pluginID))
	if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.ManagementUI == nil {
		return plugins.Snapshot{}, false
	}
	if strings.TrimSpace(snapshot.PackageRootPath) == "" || len(snapshot.ManagementUI.Pages) == 0 || strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry) == "" {
		return plugins.Snapshot{}, false
	}
	return snapshot, true
}

func (h *PluginManagementUIHandlers) resolveSettingsSnapshot(w http.ResponseWriter, r *http.Request) (plugins.Snapshot, bool) {
	if h.plugins == nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
		return plugins.Snapshot{}, false
	}

	pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
	snapshot, ok := h.plugins.Get(pluginID)
	if !ok {
		httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
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
		httpapi.WriteError(w, r, http.StatusConflict, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", details)
		return plugins.Snapshot{}, false
	}

	if snapshot.RegistrationState != "installed" {
		httpapi.WriteError(w, r, http.StatusConflict, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", map[string]any{
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

	if len(snapshot.ManagementUI.Pages) == 0 {
		return ""
	}
	entryDir := path.Dir(strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry))
	if entryDir == "." || entryDir == "/" {
		return filepath.Clean(snapshot.PackageRootPath)
	}
	return filepath.Clean(filepath.Join(snapshot.PackageRootPath, filepath.FromSlash(entryDir)))
}

func normalizePluginUIAssetPath(assetPath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(assetPath))
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func isPathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (!strings.HasPrefix(relativePath, "..") && relativePath != "")
}

func writeNoStoreHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Cache-Control", "no-store, max-age=0")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}
