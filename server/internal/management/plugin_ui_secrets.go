package management

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func (h *PluginManagementUIHandlers) HandlePluginSecretsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		configured, err := h.readPluginSecretStatus(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
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
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		var req pluginSecretsRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || len(req.Values) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		changed := make(map[string]struct{}, len(req.Values))
		for rawKey, value := range req.Values {
			key := strings.TrimSpace(rawKey)
			if !isPluginSecretKey(key) || value == "" {
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
		configured, err := h.readPluginSecretStatus(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{PluginID: snapshot.PluginID, ChangedKeys: sortedStringSet(changed), Configured: configured})
	}
}

func (h *PluginManagementUIHandlers) HandlePluginSecretsDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		var req pluginSecretsDeleteRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || len(req.Keys) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		changed := make(map[string]struct{}, len(req.Keys))
		for _, rawKey := range req.Keys {
			key := strings.TrimSpace(rawKey)
			if !isPluginSecretKey(key) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			if _, duplicate := changed[key]; duplicate {
				httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			if err := h.secrets.Delete(r.Context(), pluginSecretStorageKey(snapshot.PluginID, key)); err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			changed[key] = struct{}{}
		}
		configured, err := h.readPluginSecretStatus(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		for key := range changed {
			if _, exists := configured[key]; !exists {
				configured[key] = false
			}
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{PluginID: snapshot.PluginID, ChangedKeys: sortedStringSet(changed), Configured: configured})
	}
}

func (h *PluginManagementUIHandlers) readPluginSecretStatus(ctx context.Context, pluginID string) (map[string]bool, error) {
	keys, err := h.secrets.List(ctx)
	if err != nil {
		return nil, err
	}
	prefix := pluginSecretStorageKey(pluginID, "")
	values := make(map[string]bool)
	for _, storageKey := range keys {
		if !strings.HasPrefix(storageKey, prefix) {
			continue
		}
		key := strings.TrimPrefix(storageKey, prefix)
		if strings.TrimSpace(key) != "" {
			values[key] = true
		}
	}
	return values, nil
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
