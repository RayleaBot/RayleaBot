package logapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/logs", h.HandleLogsList())
	router.Get("/api/logs/{log_id}", h.HandleLogDetail())
}
