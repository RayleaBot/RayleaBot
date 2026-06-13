package pluginui

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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
