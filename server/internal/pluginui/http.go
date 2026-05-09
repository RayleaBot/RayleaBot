package pluginui

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type Deps struct {
	Plugins            *plugins.Catalog
	PluginConfig       pluginconfig.Repository
	Secrets            secrets.Store
	NotifyConfigChange func(context.Context, string)
	RefreshCommands    func(context.Context, string, map[string]any)
}

type Handlers struct {
	plugins            *plugins.Catalog
	pluginConfig       pluginconfig.Repository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
}

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		plugins:            deps.Plugins,
		pluginConfig:       deps.PluginConfig,
		secrets:            deps.Secrets,
		notifyConfigChange: deps.NotifyConfigChange,
		refreshCommands:    deps.RefreshCommands,
	}
}

func (h *Handlers) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
	router.Head("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsGet())
	router.Put("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsPut())
	router.Get("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsPut())
}

type pluginSettingsRequest struct {
	Values map[string]any `json:"values"`
}

type pluginSecretsRequest struct {
	Values      map[string]string `json:"values"`
	DeletedKeys []string          `json:"deleted_keys,omitempty"`
}

type PluginSettingsResponse struct {
	PluginID string         `json:"plugin_id"`
	Values   map[string]any `json:"values"`
}

type PluginSettingsUpdateResponse struct {
	PluginID    string         `json:"plugin_id"`
	ChangedKeys []string       `json:"changed_keys"`
	Values      map[string]any `json:"values"`
}

type PluginSecretsResponse struct {
	PluginID string            `json:"plugin_id"`
	Values   map[string]string `json:"values"`
}

type PluginSecretsUpdateResponse struct {
	PluginID    string            `json:"plugin_id"`
	ChangedKeys []string          `json:"changed_keys"`
	Values      map[string]string `json:"values"`
}

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

		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}
}

func (h *Handlers) HandlePluginSettingsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}

		values, err := h.effectiveSettings(r.Context(), snapshot)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSettingsResponse{
			PluginID: snapshot.PluginID,
			Values:   values,
		})
	}
}

func (h *Handlers) HandlePluginSettingsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.pluginConfig == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var req pluginSettingsRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || req.Values == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		changedKeys, err := h.pluginConfig.Write(r.Context(), snapshot.PluginID, req.Values)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		values, err := h.effectiveSettings(r.Context(), snapshot)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		if len(changedKeys) > 0 {
			if h.refreshCommands != nil {
				h.refreshCommands(r.Context(), snapshot.PluginID, values)
			}
			if h.notifyConfigChange != nil {
				h.notifyConfigChange(r.Context(), snapshot.PluginID)
			}
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSettingsUpdateResponse{
			PluginID:    snapshot.PluginID,
			ChangedKeys: changedKeys,
			Values:      values,
		})
	}
}

func (h *Handlers) HandlePluginSecretsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		values, err := h.readPluginSecrets(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsResponse{
			PluginID: snapshot.PluginID,
			Values:   values,
		})
	}
}

func (h *Handlers) HandlePluginSecretsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var req pluginSecretsRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || req.Values == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		changed := map[string]struct{}{}
		for key, value := range req.Values {
			key = strings.TrimSpace(key)
			if !isPluginSecretKey(key) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			sealed, err := secrets.SealString(r.Context(), h.secrets, value)
			if err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			if err := h.secrets.Set(r.Context(), pluginSecretStorageKey(snapshot.PluginID, key), sealed); err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			changed[key] = struct{}{}
		}
		for _, key := range req.DeletedKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if !isPluginSecretKey(key) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			if err := h.secrets.Delete(r.Context(), pluginSecretStorageKey(snapshot.PluginID, key)); err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			changed[key] = struct{}{}
		}

		values, err := h.readPluginSecrets(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{
			PluginID:    snapshot.PluginID,
			ChangedKeys: sortedStringSet(changed),
			Values:      values,
		})
	}
}

func (h *Handlers) effectiveSettings(ctx context.Context, snapshot plugins.Snapshot) (map[string]any, error) {
	values := cloneSettingsMap(snapshot.DefaultConfig)
	if h.pluginConfig == nil {
		return ensureSettingsMap(values), nil
	}

	persisted, err := h.pluginConfig.ReadAll(ctx, snapshot.PluginID)
	if err != nil {
		return nil, err
	}
	for key, value := range persisted {
		values[key] = cloneSettingsValue(value)
	}
	return ensureSettingsMap(values), nil
}

func (h *Handlers) readPluginSecrets(ctx context.Context, pluginID string) (map[string]string, error) {
	keys, err := h.secrets.List(ctx)
	if err != nil {
		return nil, err
	}

	prefix := pluginSecretStorageKey(pluginID, "")
	values := make(map[string]string)
	for _, storageKey := range keys {
		if !strings.HasPrefix(storageKey, prefix) {
			continue
		}
		key := strings.TrimPrefix(storageKey, prefix)
		if strings.TrimSpace(key) == "" {
			continue
		}
		value, err := h.secrets.Get(ctx, storageKey)
		if err != nil {
			return nil, err
		}
		plaintext, err := secrets.OpenString(ctx, h.secrets, value)
		if err != nil {
			return nil, err
		}
		values[key] = plaintext
	}
	return values, nil
}

func (h *Handlers) resolvePluginUISnapshot(pluginID string) (plugins.Snapshot, bool) {
	if h == nil || h.plugins == nil {
		return plugins.Snapshot{}, false
	}

	snapshot, ok := h.plugins.Get(strings.TrimSpace(pluginID))
	if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.ManagementUI == nil {
		return plugins.Snapshot{}, false
	}
	if strings.TrimSpace(snapshot.PackageRootPath) == "" || strings.TrimSpace(snapshot.ManagementUI.Entry) == "" {
		return plugins.Snapshot{}, false
	}
	return snapshot, true
}

func (h *Handlers) resolveSettingsSnapshot(w http.ResponseWriter, r *http.Request) (plugins.Snapshot, bool) {
	if h == nil || h.plugins == nil {
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
			"plugin_id":          pluginID,
			"kind":               "plugin_not_installed",
			"registration_state": snapshot.RegistrationState,
		})
		return plugins.Snapshot{}, false
	}

	return snapshot, true
}

func pluginUIAssetRoot(snapshot plugins.Snapshot) string {
	if snapshot.ManagementUI == nil || strings.TrimSpace(snapshot.PackageRootPath) == "" {
		return ""
	}

	entryDir := path.Dir(strings.TrimSpace(snapshot.ManagementUI.Entry))
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

func cloneSettingsMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneSettingsValue(value)
	}
	return cloned
}

func cloneSettingsSlice(values []any) []any {
	if len(values) == 0 {
		return []any{}
	}

	cloned := make([]any, len(values))
	for index, value := range values {
		cloned[index] = cloneSettingsValue(value)
	}
	return cloned
}

func cloneSettingsValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSettingsMap(typed)
	case []any:
		return cloneSettingsSlice(typed)
	default:
		return typed
	}
}

func ensureSettingsMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}

func pluginSecretStorageKey(pluginID, key string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":secret:" + strings.TrimSpace(key)
}

func isPluginSecretKey(key string) bool {
	return pluginSecretKeyPattern.MatchString(strings.TrimSpace(key))
}

func sortedStringSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
