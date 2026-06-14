package managementui

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

func (h *Handlers) HandlePluginSecretsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := h.resolveSettingsSnapshot(w, r)
		if !ok {
			return
		}
		if h.secrets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		values, err := h.readPluginSecrets(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsResponse{
			PluginID: snapshot.PluginID,
			Values:   values,
		})
	}
}

func (h *Handlers) HandlePluginSecretsPut() http.HandlerFunc {
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
		if err := decoder.Decode(&req); err != nil || req.Values == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		changed := map[string]struct{}{}
		for key, value := range req.Values {
			key = strings.TrimSpace(key)
			if !isPluginSecretKey(key) {
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
		for _, key := range req.DeletedKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if !isPluginSecretKey(key) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			if err := h.secrets.Delete(r.Context(), pluginSecretStorageKey(snapshot.PluginID, key)); err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			changed[key] = struct{}{}
		}

		values, err := h.readPluginSecrets(r.Context(), snapshot.PluginID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, PluginSecretsUpdateResponse{
			PluginID:    snapshot.PluginID,
			ChangedKeys: sortedStringSet(changed),
			Values:      values,
		})
	}
}

func (h *Handlers) readPluginSecrets(ctx context.Context, pluginID string) (map[string]string, error) {
	keys, err := h.secrets.List(ctx)
	if err != nil {
		return nil, err
	}

	prefix := pluginSecretStorageKey(pluginID, "")
	values := make(map[string]string)
	for _, storageKey := range keys {
		if !strings.HasPrefix(storageKey, prefix) {
			continue
		}
		key := strings.TrimPrefix(storageKey, prefix)
		if strings.TrimSpace(key) == "" {
			continue
		}
		value, err := h.secrets.Get(ctx, storageKey)
		if err != nil {
			return nil, err
		}
		plaintext, err := secrets.OpenString(ctx, h.secrets, value)
		if err != nil {
			return nil, err
		}
		values[key] = plaintext
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
