package management

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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

func (h *PluginManagementUIHandlers) effectiveSettings(ctx context.Context, snapshot plugins.Snapshot) (map[string]any, error) {
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
