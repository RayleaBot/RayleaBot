package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *renderHTTPHandlers) handleSystemRenderTemplateList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.renderer.ListTemplates(r.Context())
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		response := renderTemplateListResponse{
			Items: make([]renderTemplateSummary, 0, len(items)),
		}
		for _, item := range items {
			response.Items = append(response.Items, toRenderTemplateSummary(item))
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "template_id")
		detail, err := h.renderer.GetTemplate(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		_, source, err := h.renderer.GetTemplateSource(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}
		previewData, err := h.renderer.GetTemplatePreviewData(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, renderTemplateDetailResponse{
			Template: toRenderTemplateDetail(detail, source, previewData),
		})
	}
}
