package thirdpartyapi

import "github.com/go-chi/chi/v5"

func (h *ThirdPartyHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/third-party/accounts", h.HandleThirdPartyAccountList())
	router.Post("/api/third-party/accounts/{platform}/login/qrcode", h.HandleThirdPartyQRCodeLoginCreate())
	router.Get("/api/third-party/accounts/{platform}/login/qrcode/{login_id}", h.HandleThirdPartyQRCodeLoginPoll())
	router.Put("/api/third-party/accounts/{platform}/{account_id}", h.HandleThirdPartyAccountUpsert())
	router.Delete("/api/third-party/accounts/{platform}/{account_id}", h.HandleThirdPartyAccountDelete())
	router.Get("/api/third-party/monitors", h.HandleThirdPartyMonitorList())
	router.Get("/api/third-party/media", h.HandleThirdPartyMedia())
}
