package management

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginmarket"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type PluginStoreRoutes struct {
	Service pluginmarket.ServiceAPI
}

type pluginStoreInstallRequest struct {
	Version              string `json:"version,omitempty"`
	TrustedCodeConfirmed bool   `json:"trusted_code_confirmed"`
}

type pluginStoreRefreshResponse struct {
	Catalog pluginmarket.CatalogStatus `json:"catalog"`
}

func (routes PluginStoreRoutes) RegisterProtectedRoutes(router chi.Router) {
	if routes.Service == nil {
		return
	}
	router.Get("/api/plugin-store/plugins", routes.list())
	router.Get("/api/plugin-store/plugins/{plugin_id}", routes.detail())
	router.Post("/api/plugin-store/plugins/{plugin_id}/install", routes.install())
	router.Post("/api/plugin-store/refresh", routes.refresh())
}

func (routes PluginStoreRoutes) list() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := pluginmarket.Query{
			Text:      r.URL.Query().Get("query"),
			Publisher: r.URL.Query().Get("publisher"),
			Sort:      r.URL.Query().Get("sort"),
			Limit:     24,
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			query.Cursor = value
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 100 {
				writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			query.Limit = value
		}
		if query.Sort != "" && query.Sort != "recommended" && query.Sort != "name" && query.Sort != "updated" {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		writeJSON(w, http.StatusOK, routes.Service.List(query))
	}
}

func (routes PluginStoreRoutes) detail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		detail, ok := routes.Service.Get(pluginID)
		if !ok {
			writeError(w, r, http.StatusNotFound, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin_store_entry", "plugin_id": pluginID})
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func (routes PluginStoreRoutes) install() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pluginStoreInstallRequest
		if err := decodeStrictJSON(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		taskID, err := routes.Service.Install(r.Context(), pluginmarket.InstallRequest{
			PluginID:             chi.URLParam(r, "plugin_id"),
			Version:              strings.TrimSpace(request.Version),
			TrustedCodeConfirmed: request.TrustedCodeConfirmed,
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func (routes PluginStoreRoutes) refresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := routes.Service.Refresh(r.Context())
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, pluginStoreRefreshResponse{Catalog: status})
	}
}

func writePluginStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pluginmarket.ErrEntryNotFound):
		writeError(w, r, http.StatusNotFound, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation):
		writePluginInstallError(w, r, err)
	case errors.Is(err, tasks.ErrQueueFull):
		writePluginInstallError(w, r, err)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeCatalogUnavailable:
		writeError(w, r, http.StatusServiceUnavailable, pluginmarket.CodeCatalogUnavailable, "插件商店目录暂不可用", "errors.plugin.store_catalog_unavailable", nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeReleaseUnavailable:
		writeError(w, r, http.StatusConflict, pluginmarket.CodeReleaseUnavailable, "当前平台没有可安装的插件商店产物", "errors.plugin.store_release_unavailable", nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeIntegrityMismatch || pluginservice.InstallErrorCode(err) == pluginmarket.CodeIntegrityMismatch:
		writeError(w, r, http.StatusConflict, pluginmarket.CodeIntegrityMismatch, "插件商店产物与签名目录不一致", "errors.plugin.store_integrity_mismatch", nil)
	default:
		writePluginInstallError(w, r, err)
	}
}
