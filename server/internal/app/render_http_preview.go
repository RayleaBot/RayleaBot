package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func (h *renderHTTPHandlers) handleSystemRenderTemplatePreviewHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		templateID := chi.URLParam(r, "template_id")
		var request renderTemplatePreviewHTMLRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Data == nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		result, err := h.renderer.PreviewHTML(r.Context(), render.Request{
			Template: templateID,
			Theme:    request.Theme,
			Data:     request.Data,
		})
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, toRenderTemplatePreviewHTMLResponse(result))
	}
}
