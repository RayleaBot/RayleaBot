package protocolapi

import "github.com/go-chi/chi/v5"

func (h *ProtocolHandlers) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/protocols/onebot11/reverse-ws", h.HandleProtocolOneBot11ReverseWS())
	router.Post("/api/protocols/onebot11/webhook", h.HandleProtocolOneBot11Webhook())
}

func (h *ProtocolHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/protocols/onebot11", h.HandleProtocolOneBot11Snapshot())
	router.Get("/api/protocols/onebot11/targets", h.HandleProtocolOneBot11Targets())
	router.Post("/api/protocols/onebot11/identities/resolve", h.HandleProtocolOneBot11IdentitiesResolve())
	router.Get("/api/protocols/onebot11/compatibility", h.HandleProtocolOneBot11Compatibility())
}
