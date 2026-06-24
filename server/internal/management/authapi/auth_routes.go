package authapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterPublicRoutes(router chi.Router) {
	router.Post("/api/setup/admin", h.HandleSetupAdmin())
	router.Post("/api/session/login", h.HandleSessionLogin())
}
