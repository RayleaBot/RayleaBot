package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *bilibiliSourceHTTPHandlers) handleBilibiliQRCodeLoginCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Create(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录创建失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, bilibiliQRCodeLoginCreateResponse{
			LoginID:   session.LoginID,
			QRCodeURL: session.QRCodeURL,
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			State:     session.State,
		})
	}
}

func (h *bilibiliSourceHTTPHandlers) handleBilibiliQRCodeLoginPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Poll(r.Context(), chi.URLParam(r, "login_id"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "Bilibili 扫码登录状态不可用", "errors.platform.invalid_request", nil)
			return
		}
		var cookie *string
		if session.Cookie != "" {
			cookie = &session.Cookie
		}
		var account *thirdPartyAccountProfile
		if session.Account.UID != "" || session.Account.Nickname != "" || session.Account.AvatarURL != "" {
			account = &thirdPartyAccountProfile{
				UID:       session.Account.UID,
				Nickname:  session.Account.Nickname,
				AvatarURL: session.Account.AvatarURL,
			}
		}
		httpapi.WriteJSON(w, http.StatusOK, bilibiliQRCodeLoginPollResponse{
			LoginID:   session.LoginID,
			State:     session.State,
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			Cookie:    cookie,
			Account:   account,
		})
	}
}

type bilibiliQRCodeLoginCreateResponse struct {
	LoginID   string `json:"login_id"`
	QRCodeURL string `json:"qrcode_url"`
	ExpiresAt string `json:"expires_at"`
	State     string `json:"state"`
}

type bilibiliQRCodeLoginPollResponse struct {
	LoginID   string                    `json:"login_id"`
	State     string                    `json:"state"`
	ExpiresAt string                    `json:"expires_at"`
	Cookie    *string                   `json:"cookie"`
	Account   *thirdPartyAccountProfile `json:"account"`
}
