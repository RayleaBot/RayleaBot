package management

import (
	"context"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
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

func (h *PluginManagementUIHandlers) HandlePluginSettingsPut() http.HandlerFunc {
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
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil || req.Values == nil {
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
				h.notifyConfigChange(r.Context(), snapshot.PluginID, values, changedKeys)
			}
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSettingsUpdateResponse{
			PluginID:    snapshot.PluginID,
			ChangedKeys: changedKeys,
			Values:      values,
		})
	}
}

func (h *PluginManagementUIHandlers) effectiveSettings(ctx context.Context, snapshot plugins.Snapshot) (map[string]any, error) {
	if h.pluginConfig == nil {
		return pluginstore.MergeValues(snapshot.DefaultConfig, nil), nil
	}

	persisted, err := h.pluginConfig.ReadAll(ctx, snapshot.PluginID)
	if err != nil {
		return nil, err
	}
	return pluginstore.MergeValues(snapshot.DefaultConfig, persisted), nil
}
