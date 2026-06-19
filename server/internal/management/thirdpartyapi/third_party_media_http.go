package thirdpartyapi

import (
	"errors"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/media"
)

func (h *ThirdPartyHandlers) HandleThirdPartyMedia() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resource, err := media.Fetch(r.Context(), h.mediaClient, r.URL.Query().Get("url"))
		if err != nil {
			writeThirdPartyMediaError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", resource.ContentType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resource.Body)
	}
}

func writeThirdPartyMediaError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, media.ErrUnsupportedURL) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方媒体地址不受支持", "errors.platform.invalid_request", nil)
		return
	}
	if errors.Is(err, media.ErrUnsupportedContentType) {
		httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体响应格式不正确", "errors.platform.internal_error", nil)
		return
	}
	httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体读取失败", "errors.platform.internal_error", nil)
}
