package managementui

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (h *Handlers) resolvePluginUISnapshot(pluginID string) (plugins.Snapshot, bool) {
	if h == nil || h.plugins == nil {
		return plugins.Snapshot{}, false
	}

	snapshot, ok := h.plugins.Get(strings.TrimSpace(pluginID))
	if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.ManagementUI == nil {
		return plugins.Snapshot{}, false
	}
	if strings.TrimSpace(snapshot.PackageRootPath) == "" || len(snapshot.ManagementUI.Pages) == 0 || strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry) == "" {
		return plugins.Snapshot{}, false
	}
	return snapshot, true
}

func (h *Handlers) resolveSettingsSnapshot(w http.ResponseWriter, r *http.Request) (plugins.Snapshot, bool) {
	if h == nil || h.plugins == nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
		return plugins.Snapshot{}, false
	}

	pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
	snapshot, ok := h.plugins.Get(pluginID)
	if !ok {
		httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"resource_type": "plugin",
			"plugin_id":     pluginID,
		})
		return plugins.Snapshot{}, false
	}

	if !snapshot.Valid {
		details := map[string]any{
			"plugin_id": pluginID,
		}
		if snapshot.DisplayState == "conflict" {
			details["kind"] = "plugin_id_conflict"
			details["manifest_paths"] = append([]string(nil), snapshot.ConflictPaths...)
			details["source_roots"] = append([]string(nil), snapshot.SourceRoots...)
		} else {
			details["kind"] = "invalid_manifest"
			details["manifest_path"] = snapshot.ManifestPath
			details["validation_summary"] = snapshot.ValidationSummary
		}
		httpapi.WriteError(w, r, http.StatusConflict, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", details)
		return plugins.Snapshot{}, false
	}

	if snapshot.RegistrationState != "installed" {
		httpapi.WriteError(w, r, http.StatusConflict, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", map[string]any{
			"plugin_id": pluginID,
			"kind":      "plugin_not_installed",
			"installed": false,
		})
		return plugins.Snapshot{}, false
	}

	return snapshot, true
}
