package managementhttp

import (
	"context"
	"net/http"
	"strings"

	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type thirdPartyMonitorService interface {
	MonitorSnapshot(context.Context) (bilibilisource.MonitorSnapshot, error)
}

func (h *ThirdPartyHandlers) HandleThirdPartyMonitorList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform := strings.TrimSpace(r.URL.Query().Get("platform"))
		if platform == "" {
			platform = thirdparty.PlatformBilibili
		}
		if platform != thirdparty.PlatformBilibili {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方监控平台不正确", "errors.platform.invalid_request", nil)
			return
		}
		if h.monitors == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方监控读取失败", "errors.platform.internal_error", nil)
			return
		}
		snapshot, err := h.monitors.MonitorSnapshot(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方监控读取失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyMonitorsResponseFrom(snapshot))
	}
}
