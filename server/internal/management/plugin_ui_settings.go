package management

import (
	"errors"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

type pluginSettingsRequest struct {
	Values map[string]any `json:"values"`
}

type pluginSecretsRequest struct {
	Values map[string]string `json:"values"`
}

type pluginSecretsDeleteRequest struct {
	Keys []string `json:"keys"`
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
	PluginID   string          `json:"plugin_id"`
	Configured map[string]bool `json:"configured"`
}

type PluginSecretsUpdateResponse struct {
	PluginID    string          `json:"plugin_id"`
	ChangedKeys []string        `json:"changed_keys"`
	Configured  map[string]bool `json:"configured"`
}

func (h *PluginManagementUIHandlers) HandlePluginSettingsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}

		values, err := h.settings.Read(r.Context(), snapshot.PluginID)
		if err != nil {
			writePluginSettingsError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSettingsResponse{
			PluginID: snapshot.PluginID,
			Values:   values,
		})
	}
}

func (h *PluginManagementUIHandlers) HandlePluginSettingsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		var req pluginSettingsRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil || req.Values == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}

		result, err := h.settings.Write(r.Context(), snapshot.PluginID, req.Values)
		if err != nil {
			writePluginSettingsError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSettingsUpdateResponse{
			PluginID:    snapshot.PluginID,
			ChangedKeys: result.ChangedKeys,
			Values:      result.Values,
		})
	}
}

func writePluginSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var applyErr *settings.ApplyError
	switch {
	case errors.As(err, &applyErr):
		httpapi.WriteError(w, r, errorcodes.PluginSettingsApplyFailed, applyErr.Details())
	case errors.Is(err, settings.ErrInvalidValues):
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
	case errors.Is(err, settings.ErrPluginNotFound):
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
	default:
		httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
	}
}
