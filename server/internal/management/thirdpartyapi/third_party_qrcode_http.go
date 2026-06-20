package thirdpartyapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	thirdpartylogin "github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdpartylogin"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
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
	Cookie    *string                   `json:"cookie"`
	Account   *thirdPartyAccountProfile `json:"account"`
}

func thirdPartyQRCodeLoginCreateResponseFrom(result thirdpartylogin.CreateResult) thirdPartyQRCodeLoginCreateResponse {
	return thirdPartyQRCodeLoginCreateResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: timeString(result.ExpiresAt),
		State:     result.State,
	}
}

func thirdPartyQRCodeLoginPollResponseFrom(result thirdpartylogin.PollResult) thirdPartyQRCodeLoginPollResponse {
	var cookie *string
	if result.Cookie != "" {
		cookie = &result.Cookie
	}
	var account *thirdPartyAccountProfile
	if result.Account.UID != "" || result.Account.Nickname != "" || result.Account.AvatarURL != "" {
		account = &thirdPartyAccountProfile{
			UID:       result.Account.UID,
			Nickname:  result.Account.Nickname,
			AvatarURL: result.Account.AvatarURL,
		}
	}
	return thirdPartyQRCodeLoginPollResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		State:     result.State,
		ExpiresAt: timeString(result.ExpiresAt),
		Cookie:    cookie,
		Account:   account,
	}
}

func writeThirdPartyQRCodeLoginError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, thirdparty.ErrInvalidAccount) || errors.Is(err, thirdpartylogin.ErrUnsupportedPlatform) || errors.Is(err, thirdpartylogin.ErrLoginSessionNotFound) {
		httpapi.WriteError(w, r, http.StatusBadRequest, codeInvalidRequest, "三方扫码登录参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, codeInternalError, "三方扫码登录失败", "errors.platform.internal_error", nil)
}
