package bilibiliapi

import "github.com/go-chi/chi/v5"

func (h *BilibiliHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Post("/api/bilibili/login/qrcode", h.HandleBilibiliQRCodeLoginCreate())
	router.Get("/api/bilibili/login/qrcode/{login_id}", h.HandleBilibiliQRCodeLoginPoll())
	router.Get("/api/bilibili/users/resolve", h.HandleBilibiliUserResolve())
	router.Get("/api/bilibili/source/status", h.HandleBilibiliSourceStatus())
	router.Post("/api/bilibili/source/restart", h.HandleBilibiliSourceRestart())
}
