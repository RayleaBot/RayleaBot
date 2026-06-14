package managementhttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *RenderHandlers) HandleSystemRenderTemplateAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "render_template_asset",
			})
			return
		}

		templateID := chi.URLParam(r, "template_id")
		asset, err := h.renderer.LookupTemplateAsset(r.Context(), templateID, r.URL.Query().Get("path"))
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		http.ServeFile(w, r, asset.Path)
	}
}
