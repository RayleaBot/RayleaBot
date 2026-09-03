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

type pluginStoreInspectionRequest struct {
	SourceID string `json:"source_id"`
}

type pluginStoreInspectionResponse struct {
	Inspection           pluginInstallInspectionResponse `json:"inspection"`
	ConfirmationRequired bool                            `json:"confirmation_required"`
	ConfirmationReasons  []string                        `json:"confirmation_reasons"`
}

type pluginStoreInstallRequest struct {
	InspectionID         string `json:"inspection_id"`
	PackageSHA256        string `json:"package_sha256"`
	TrustedCodeConfirmed bool   `json:"trusted_code_confirmed"`
}

type pluginStoreSourcesResponse struct {
	Items []pluginmarket.SourceView `json:"items"`
}

func (routes PluginStoreRoutes) RegisterProtectedRoutes(router chi.Router) {
	if routes.Service == nil {
		return
	}
	router.Get("/api/plugin-store/plugins", routes.list())
	router.Get("/api/plugin-store/plugins/{plugin_id}", routes.detail())
	router.Post("/api/plugin-store/plugins/{plugin_id}/inspect", routes.inspect())
	router.Post("/api/plugin-store/plugins/{plugin_id}/install", routes.install())
	router.Get("/api/plugin-store/sources", routes.listSources())
	router.Post("/api/plugin-store/sources", routes.createSource())
	router.Put("/api/plugin-store/sources/{source_id}", routes.updateSource())
	router.Delete("/api/plugin-store/sources/{source_id}", routes.deleteSource())
	router.Post("/api/plugin-store/sources/{source_id}/refresh", routes.refreshSource())
}

func (routes PluginStoreRoutes) list() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := pluginmarket.Query{
			SourceID: r.URL.Query().Get("source_id"),
			Text:     r.URL.Query().Get("query"),
			Sort:     r.URL.Query().Get("sort"),
			Limit:    24,
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
		result, err := routes.Service.List(query)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func (routes PluginStoreRoutes) detail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		detail, ok := routes.Service.Get(r.URL.Query().Get("source_id"), pluginID)
		if !ok {
			writeError(w, r, http.StatusNotFound, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin_store_entry", "plugin_id": pluginID})
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func (routes PluginStoreRoutes) inspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pluginStoreInspectionRequest
		if err := decodeStrictJSON(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		result, err := routes.Service.Inspect(r.Context(), pluginmarket.InspectionRequest{
			SourceID: strings.TrimSpace(request.SourceID),
			PluginID: chi.URLParam(r, "plugin_id"),
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, pluginStoreInspectionResponse{
			Inspection:           buildInstallInspectionResponse(result.Inspection),
			ConfirmationRequired: result.ConfirmationRequired,
			ConfirmationReasons:  append([]string(nil), result.ConfirmationReasons...),
		})
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
			InspectionID:         strings.TrimSpace(request.InspectionID),
			PackageSHA256:        strings.TrimSpace(request.PackageSHA256),
			TrustedCodeConfirmed: request.TrustedCodeConfirmed,
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func (routes PluginStoreRoutes) listSources() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, pluginStoreSourcesResponse{Items: routes.Service.Sources()})
	}
}

func (routes PluginStoreRoutes) createSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input pluginmarket.SourceInput
		if err := decodeStrictJSON(r, &input); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		source, err := routes.Service.CreateSource(r.Context(), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, source)
	}
}

func (routes PluginStoreRoutes) updateSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input pluginmarket.SourceInput
		if err := decodeStrictJSON(r, &input); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		source, err := routes.Service.UpdateSource(r.Context(), chi.URLParam(r, "source_id"), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, source)
	}
}

func (routes PluginStoreRoutes) deleteSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := routes.Service.DeleteSource(r.Context(), chi.URLParam(r, "source_id")); err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (routes PluginStoreRoutes) refreshSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, err := routes.Service.Refresh(r.Context(), chi.URLParam(r, "source_id"))
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, source)
	}
}

func writePluginStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pluginmarket.ErrEntryNotFound), errors.Is(err, pluginmarket.ErrSourceNotFound):
		writeError(w, r, http.StatusNotFound, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", nil)
	case errors.Is(err, pluginmarket.ErrSourceImmutable):
		writeError(w, r, http.StatusConflict, pluginCodeInvalidRequest, "官方插件源不能修改或删除", "errors.platform.invalid_request", nil)
	case errors.Is(err, pluginmarket.ErrSourceConflict):
		writeError(w, r, http.StatusConflict, pluginCodeInvalidRequest, "插件源地址已存在", "errors.platform.invalid_request", nil)
	case errors.Is(err, pluginmarket.ErrSourceInvalid):
		writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "插件源名称或地址不合法", "errors.platform.invalid_request", nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation),
		errors.Is(err, plugins.ErrInstallInspectionRequired),
		errors.Is(err, plugins.ErrInstallInspectionExpired),
		errors.Is(err, plugins.ErrInstallDigestMismatch),
		errors.Is(err, tasks.ErrQueueFull):
		writePluginInstallError(w, r, err)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeCatalogUnavailable:
		writeError(w, r, http.StatusServiceUnavailable, pluginmarket.CodeCatalogUnavailable, "插件源暂不可用，已保留上次成功结果", "errors.plugin.store_catalog_unavailable", nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeReleaseUnavailable:
		writeError(w, r, http.StatusConflict, pluginmarket.CodeReleaseUnavailable, "当前平台没有可安装的插件产物", "errors.plugin.store_release_unavailable", nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeIntegrityMismatch || pluginservice.InstallErrorCode(err) == pluginmarket.CodeIntegrityMismatch:
		writeError(w, r, http.StatusConflict, pluginmarket.CodeIntegrityMismatch, "插件产物摘要或身份与目录不一致", "errors.plugin.store_integrity_mismatch", nil)
	default:
		writePluginInstallError(w, r, err)
	}
}
