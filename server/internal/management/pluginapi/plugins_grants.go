package pluginapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi/view"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func registerPluginGrantRoutes(router chi.Router, catalog plugins.CatalogView, repo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	router.Get("/api/plugins/{plugin_id}/grants", newListGrantsHandler(catalog, repo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/grants", newGrantHandler(catalog, repo))
	router.Delete("/api/plugins/{plugin_id}/grants/{capability}", newRevokeGrantHandler(catalog, repo))
}

func newListGrantsHandler(catalog plugins.CatalogView, repo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		persisted, err := loadPersistedGrants(r.Context(), repo, pluginID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		effective := plugins.ComputeEffectiveGrants(snapshot, providedAutoGrantCapabilities(autoGrantProvider), persisted)
		writeJSON(w, http.StatusOK, view.GrantsListResponse{Items: view.BuildGrantResponses(effective)})
	}
}

func newGrantHandler(catalog plugins.CatalogView, repo plugins.GrantRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if repo == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		var req grantRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || req.Capability == "" {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if !capabilityNamePattern.MatchString(req.Capability) {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "capability 名称格式不合法", "errors.platform.invalid_request", map[string]any{"capability": req.Capability})
			return
		}
		if !view.IsCapabilityDeclared(snapshot, req.Capability) {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "capability 未在插件 manifest 中声明", "errors.platform.invalid_request", map[string]any{"capability": req.Capability, "plugin_id": pluginID})
			return
		}
		expiresAt, err := parseGrantRequestExpiry(req.ExpiresAt)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		grant := plugins.PluginGrant{
			PluginID:   pluginID,
			Capability: req.Capability,
			ScopeJSON:  view.BuildScopeJSON(snapshot),
			GrantedAt:  time.Now().UTC(),
			ExpiresAt:  expiresAt,
		}
		if err := repo.SaveGrant(r.Context(), grant); err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		grantedAt := grant.GrantedAt.UTC().Format(time.RFC3339)
		response := view.GrantResponse{
			PluginID:   grant.PluginID,
			Capability: grant.Capability,
			GrantedAt:  &grantedAt,
			Source:     string(plugins.GrantSourcePersisted),
		}
		if grant.ExpiresAt != nil {
			value := grant.ExpiresAt.UTC().Format(time.RFC3339)
			response.ExpiresAt = &value
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func newRevokeGrantHandler(catalog plugins.CatalogView, repo plugins.GrantRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		capability := chi.URLParam(r, "capability")
		if _, ok := catalog.Get(pluginID); !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if repo == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if err := repo.DeleteGrant(r.Context(), pluginID, capability); err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
