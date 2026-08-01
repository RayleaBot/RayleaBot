package management

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
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

type PluginUIOriginOptions struct {
	OriginTemplate string
	ServerPort     int
	AdminOrigins   []string
}

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
		plugins:            deps.Plugins,
		pluginConfig:       deps.PluginConfig,
		secrets:            deps.Secrets,
		notifyConfigChange: deps.NotifyConfigChange,
		refreshCommands:    deps.RefreshCommands,
		actionInvoker:      deps.ActionInvoker,
	}
}

func (h *PluginManagementUIHandlers) RegisterPublicRoutes(router chi.Router) {
	// Plugin UI assets are intentionally not mounted on the admin router. The
	// app wraps that router with IsolatedOriginHandler so plugin origins can
	// never inherit management API routes, sessions, or CORS behavior.
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

func (h *PluginManagementUIHandlers) IsolatedOriginHandler(next http.Handler, options PluginUIOriginOptions) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolvePluginUIOrigin(r.Host, options)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		h.servePluginUIAsset(w, r, snapshot, options)
	})
}

func (h *PluginManagementUIHandlers) servePluginUIAsset(w http.ResponseWriter, r *http.Request, snapshot plugins.Snapshot, options PluginUIOriginOptions) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(path.Clean("/"+r.URL.Path), "/api/") || strings.HasPrefix(path.Clean("/"+r.URL.Path), "/ws/") {
		http.NotFound(w, r)
		return
	}

	assetRoot := pluginUIAssetRoot(snapshot)
	if assetRoot == "" {
		http.NotFound(w, r)
		return
	}
	assetPath := normalizePluginUIAssetPath(r.URL.Path)
	if assetPath == "" {
		assetPath = strings.TrimPrefix(strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry), "ui/")
	}
	assetFile := filepath.Clean(filepath.Join(assetRoot, filepath.FromSlash(assetPath)))
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

	writePluginUIHeaders(w, assetPath, options.AdminOrigins)
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *PluginManagementUIHandlers) resolvePluginUIOrigin(requestHost string, options PluginUIOriginOptions) (plugins.Snapshot, bool) {
	if h == nil || h.plugins == nil {
		return plugins.Snapshot{}, false
	}
	requestHost = strings.TrimSpace(requestHost)
	for _, snapshot := range h.plugins.List() {
		if !pluginUISnapshotReady(snapshot) {
			continue
		}
		origin, err := PluginUIOrigin(snapshot.PluginID, options)
		if err != nil {
			continue
		}
		parsed, err := url.Parse(origin)
		if err == nil && strings.EqualFold(parsed.Host, requestHost) {
			return snapshot, true
		}
	}
	return plugins.Snapshot{}, false
}

func PluginUIOrigin(pluginID string, options PluginUIOriginOptions) (string, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	digest := sha256.Sum256([]byte(pluginID))
	pluginHost := fmt.Sprintf("p-%x", digest[:8])
	template := strings.TrimSpace(options.OriginTemplate)
	if template == "" {
		if options.ServerPort < 1 || options.ServerPort > 65535 {
			return "", fmt.Errorf("valid server port is required")
		}
		template = "http://{plugin_host}.plugins.localhost:" + strconv.Itoa(options.ServerPort)
	}
	if !strings.Contains(template, "{plugin_host}") {
		return "", fmt.Errorf("plugin UI origin template must contain {plugin_host}")
	}
	rendered := strings.ReplaceAll(template, "{plugin_host}", pluginHost)
	parsed, err := url.Parse(rendered)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("plugin UI origin template must render to an HTTP(S) origin")
	}
	for _, rawAdminOrigin := range options.AdminOrigins {
		adminOrigin, parseErr := url.Parse(strings.TrimSpace(rawAdminOrigin))
		if parseErr == nil && sameHTTPOrigin(parsed, adminOrigin) {
			return "", fmt.Errorf("plugin UI origin must differ from the admin origin")
		}
	}
	return parsed.String(), nil
}

func sameHTTPOrigin(left, right *url.URL) bool {
	if left == nil || right == nil || !strings.EqualFold(left.Scheme, right.Scheme) || !strings.EqualFold(left.Hostname(), right.Hostname()) {
		return false
	}
	effectivePort := func(origin *url.URL) string {
		if port := origin.Port(); port != "" {
			return port
		}
		if strings.EqualFold(origin.Scheme, "https") {
			return "443"
		}
		if strings.EqualFold(origin.Scheme, "http") {
			return "80"
		}
		return ""
	}
	return effectivePort(left) == effectivePort(right)
}

func (h *PluginManagementUIHandlers) resolvePluginUISnapshot(pluginID string) (plugins.Snapshot, bool) {
	if h.plugins == nil {
		return plugins.Snapshot{}, false
	}

	snapshot, ok := h.plugins.Get(strings.TrimSpace(pluginID))
	if !ok || !pluginUISnapshotReady(snapshot) {
		return plugins.Snapshot{}, false
	}
	if strings.TrimSpace(snapshot.PackageRootPath) == "" || len(snapshot.ManagementUI.Pages) == 0 || strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry) == "" {
		return plugins.Snapshot{}, false
	}
	return snapshot, true
}

func pluginUISnapshotReady(snapshot plugins.Snapshot) bool {
	return snapshot.Valid && snapshot.RegistrationState == "installed" && snapshot.ArtifactVersion == "1" &&
		snapshot.ArtifactUIAvailable && snapshot.ManagementUI != nil && strings.TrimSpace(snapshot.PackageRootPath) != "" &&
		len(snapshot.ManagementUI.Pages) > 0 && strings.HasPrefix(strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry), "ui/")
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

	if len(snapshot.ManagementUI.Pages) == 0 || !strings.HasPrefix(strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry), "ui/") {
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

func isPathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (!strings.HasPrefix(relativePath, "..") && relativePath != "")
}

func writePluginUIHeaders(w http.ResponseWriter, assetPath string, adminOrigins []string) {
	header := w.Header()
	if hashedPluginUIAssetPattern.MatchString(path.Base(assetPath)) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-store, max-age=0")
	}
	frameAncestors := "'none'"
	if len(adminOrigins) > 0 {
		frameAncestors = strings.Join(adminOrigins, " ")
	}
	header.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors "+frameAncestors)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
}
