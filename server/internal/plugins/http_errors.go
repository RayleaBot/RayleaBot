package plugins

import (
	"errors"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, ErrPluginNotFound) {
		writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, ErrPluginNotInDeadLetter) {
		writeError(w, r, 409, "plugin.not_in_dead_letter", "插件当前不处于 dead_letter 状态", "errors.plugin.not_in_dead_letter", map[string]any{"plugin_id": pluginID})
		return
	}
	if errors.Is(err, ErrStateConflict) {
		writeError(w, r, 409, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
		return
	}
	var permissionPending *PermissionPendingError
	if errors.As(err, &permissionPending) {
		details := map[string]any{
			"plugin_id": pluginID,
		}
		if len(permissionPending.MissingCapabilities) > 0 {
			details["missing_capabilities"] = append([]string(nil), permissionPending.MissingCapabilities...)
		}
		if permissionPending.ScopeChanged {
			details["scope_changed"] = true
		}
		writeError(w, r, 409, "plugin.permission_pending", "插件所需能力尚未获批", "errors.plugin.permission_pending", details)
		return
	}

	writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
