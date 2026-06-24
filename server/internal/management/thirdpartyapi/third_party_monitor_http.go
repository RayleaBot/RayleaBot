package thirdpartyapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
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
		normalized, err := thirdparty.NormalizePlatform(platform)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方监控平台不正确", "errors.platform.invalid_request", nil)
			return
		}
		if normalized != thirdparty.PlatformBilibili {
			httpapi.WriteJSON(w, http.StatusOK, emptyThirdPartyMonitorsResponse(normalized, time.Now().UTC()))
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
