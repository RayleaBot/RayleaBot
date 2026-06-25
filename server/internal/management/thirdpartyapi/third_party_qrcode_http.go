package thirdpartyapi

import (
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func (h *ThirdPartyHandlers) HandleThirdPartyQRCodeLoginCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, codeInternalError, "三方扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		result, err := h.qrLogin.Create(r.Context(), chi.URLParam(r, "platform"))
		if err != nil {
			writeThirdPartyQRCodeLoginError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyQRCodeLoginCreateResponseFrom(result))
	}
}

func (h *ThirdPartyHandlers) HandleThirdPartyQRCodeLoginPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, codeInternalError, "三方扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		result, err := h.qrLogin.Poll(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "login_id"))
		if err != nil {
			writeThirdPartyQRCodeLoginError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyQRCodeLoginPollResponseFrom(result))
	}
}

type thirdPartyQRCodeLoginCreateResponse struct {
	Platform  string `json:"platform"`
	LoginID   string `json:"login_id"`
	QRCodeURL string `json:"qrcode_url"`
	ExpiresAt string `json:"expires_at"`
	State     string `json:"state"`
}

type thirdPartyQRCodeLoginPollResponse struct {
	Platform  string                    `json:"platform"`
	LoginID   string                    `json:"login_id"`
	State     string                    `json:"state"`
	ExpiresAt string                    `json:"expires_at"`
	Account   *thirdPartyAccountSummary `json:"account"`
}

func thirdPartyQRCodeLoginCreateResponseFrom(result qrcode.CreateResult) thirdPartyQRCodeLoginCreateResponse {
	return thirdPartyQRCodeLoginCreateResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: timeString(result.ExpiresAt),
		State:     result.State,
	}
}

func thirdPartyQRCodeLoginPollResponseFrom(result qrcode.PollResult) thirdPartyQRCodeLoginPollResponse {
	var account *thirdPartyAccountSummary
	if result.SavedAccount != nil {
		summary := accountSummary(*result.SavedAccount)
		account = &summary
	}
	return thirdPartyQRCodeLoginPollResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		State:     result.State,
		ExpiresAt: timeString(result.ExpiresAt),
		Account:   account,
	}
}

func writeThirdPartyQRCodeLoginError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, thirdparty.ErrInvalidAccount) || errors.Is(err, qrcode.ErrUnsupportedPlatform) || errors.Is(err, qrcode.ErrLoginSessionNotFound) {
		httpapi.WriteError(w, r, http.StatusBadRequest, codeInvalidRequest, "三方扫码登录参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteDomainError(w, r, &httpapi.DomainError{
		Code:        "platform.upstream_request_failed",
		HTTPStatus:  http.StatusBadGateway,
		SafeMessage: "三方扫码登录暂时不可用",
		MessageKey:  "errors.platform.upstream_request_failed",
		Details:     map[string]any{"reason": thirdPartyQRCodeLoginErrorReason(err)},
		Cause:       err,
	})
}

func thirdPartyQRCodeLoginErrorReason(err error) string {
	if errors.Is(err, qrcode.ErrLoginCredentialMissing) {
		return "credential_missing"
	}
	return "upstream_failed"
}
