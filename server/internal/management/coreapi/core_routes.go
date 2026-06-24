package coreapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/setup/status", h.HandleSetupStatus())
	router.Get("/api/launcher/status", h.HandleLauncherStatus())
	router.Post("/api/launcher/shutdown", h.HandleLauncherShutdown())
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	router.Delete("/api/session", h.HandleSessionLogout())
	router.Get("/api/system/status", h.HandleSystemStatus())
	router.Post("/api/system/shutdown", h.HandleSystemShutdown())
}
