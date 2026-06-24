package bilibiliapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func (h *BilibiliHandlers) HandleBilibiliQRCodeLoginCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Create(r.Context(), thirdparty.PlatformBilibili)
		if err != nil {
			writeBilibiliQRCodeLoginError(w, r, err)
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

func (h *BilibiliHandlers) HandleBilibiliQRCodeLoginPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Poll(r.Context(), thirdparty.PlatformBilibili, chi.URLParam(r, "login_id"))
		if err != nil {
			if !errors.Is(err, common.ErrLoginSessionNotFound) {
				writeBilibiliQRCodeLoginError(w, r, err)
				return
			}
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "Bilibili 扫码登录状态不可用", "errors.platform.invalid_request", nil)
			return
		}
		var account *thirdPartyAccountSummary
		if session.SavedAccount != nil {
			summary := accountSummary(*session.SavedAccount)
			account = &summary
		}
		httpapi.WriteJSON(w, http.StatusOK, bilibiliQRCodeLoginPollResponse{
			LoginID:   session.LoginID,
			State:     session.State,
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
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
	Account   *thirdPartyAccountSummary `json:"account"`
}

func writeBilibiliQRCodeLoginError(w http.ResponseWriter, r *http.Request, err error) {
	httpapi.WriteDomainError(w, r, &httpapi.DomainError{
		Code:        "platform.upstream_request_failed",
		HTTPStatus:  http.StatusBadGateway,
		SafeMessage: "Bilibili 扫码登录暂时不可用",
		MessageKey:  "errors.platform.upstream_request_failed",
		Details:     map[string]any{"reason": bilibiliQRCodeLoginErrorReason(err)},
		Cause:       err,
	})
}

func bilibiliQRCodeLoginErrorReason(err error) string {
	if errors.Is(err, common.ErrLoginCredentialMissing) {
		return "credential_missing"
	}
	return "upstream_failed"
}
