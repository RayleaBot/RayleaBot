package renderapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/system/render/templates", h.HandleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", h.HandleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", h.HandleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", h.HandleSystemRenderTemplateDetail())
}
