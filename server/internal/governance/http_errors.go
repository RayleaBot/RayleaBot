package governance

import (
	"errors"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func writeGovernanceError(w http.ResponseWriter, r *http.Request, err error, entryType, targetID string) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"entry_type": entryType,
			"target_id":  targetID,
		})
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
	}
}
