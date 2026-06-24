package configapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/config", h.HandleConfigGet())
	router.Put("/api/config", h.HandleConfigPut())
}
