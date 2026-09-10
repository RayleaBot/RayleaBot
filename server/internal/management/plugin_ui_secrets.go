package management

import (
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"net/http"
)

func (h *PluginManagementUIHandlers) HandlePluginSecretsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		configured, err := h.settings.SecretStatus(r.Context(), snapshot.PluginID)
		if err != nil {
			writePluginSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsResponse{PluginID: snapshot.PluginID, Configured: configured})
	}
}

func (h *PluginManagementUIHandlers) HandlePluginSecretsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		var req pluginSecretsRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		result, err := h.settings.SetSecrets(r.Context(), snapshot.PluginID, req.Values)
		if err != nil {
			writePluginSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{PluginID: snapshot.PluginID, ChangedKeys: result.ChangedKeys, Configured: result.Configured})
	}
}

func (h *PluginManagementUIHandlers) HandlePluginSecretsDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		var req pluginSecretsDeleteRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		result, err := h.settings.DeleteSecrets(r.Context(), snapshot.PluginID, req.Keys)
		if err != nil {
			writePluginSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{PluginID: snapshot.PluginID, ChangedKeys: result.ChangedKeys, Configured: result.Configured})
	}
}
