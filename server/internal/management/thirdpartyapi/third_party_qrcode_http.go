package thirdpartyapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
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
	Cookie    *string                   `json:"cookie"`
	Account   *thirdPartyAccountProfile `json:"account"`
}

func thirdPartyQRCodeLoginCreateResponseFrom(result common.CreateResult) thirdPartyQRCodeLoginCreateResponse {
	return thirdPartyQRCodeLoginCreateResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: timeString(result.ExpiresAt),
		State:     result.State,
	}
}

func thirdPartyQRCodeLoginPollResponseFrom(result common.PollResult) thirdPartyQRCodeLoginPollResponse {
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
	if errors.Is(err, thirdparty.ErrInvalidAccount) || errors.Is(err, common.ErrUnsupportedPlatform) || errors.Is(err, common.ErrLoginSessionNotFound) {
		httpapi.WriteError(w, r, http.StatusBadRequest, codeInvalidRequest, "三方扫码登录参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	msg := strings.TrimSpace(err.Error())
	code := classifyQRCodeLoginErrorCode(msg)
	// Include the actual error in the user-visible message so the user can diagnose.
	platformLabel := classifyQRCodeLoginErrorPlatform(msg)
	httpapi.WriteError(w, r, http.StatusInternalServerError, code,
		platformLabel+"扫码登录失败: "+msg,
		"errors.platform.qrcode_login_failed", nil)
}

func classifyQRCodeLoginErrorPlatform(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "douyin"):
		return "抖音"
	case strings.Contains(lower, "netease"):
		return "网易云音乐"
	case strings.Contains(lower, "weibo"):
		return "微博"
	default:
		return ""
	}
}

// classifyQRCodeLoginErrorCode maps error messages to specific error codes
// so the frontend and logs can distinguish different failure modes.
func classifyQRCodeLoginErrorCode(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	// Douyin errors
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "browser"):
		return "platform.douyin.browser_error"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "http"):
		return "platform.douyin.http_error"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "missing token"):
		return "platform.douyin.missing_token"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "create failed"):
		return "platform.douyin.create_failed"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "poll failed"):
		return "platform.douyin.poll_failed"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "login cookie"):
		return "platform.douyin.no_login_cookie"
	case strings.Contains(lower, "douyin") && strings.Contains(lower, "ticket"):
		return "platform.douyin.missing_ticket"
	case strings.Contains(lower, "douyin"):
		return "platform.douyin.error"

	// NetEase errors
	case strings.Contains(lower, "netease") && strings.Contains(lower, "csrf"):
		return "platform.netease.missing_csrf"
	case strings.Contains(lower, "netease") && strings.Contains(lower, "create code"):
		return "platform.netease.create_failed"
	case strings.Contains(lower, "netease") && strings.Contains(lower, "poll code"):
		return "platform.netease.poll_failed"
	case strings.Contains(lower, "netease") && strings.Contains(lower, "cookies"):
		return "platform.netease.no_login_cookie"
	case strings.Contains(lower, "netease") && strings.Contains(lower, "profile"):
		return "platform.netease.profile_unavailable"
	case strings.Contains(lower, "netease"):
		return "platform.netease.error"

	// Weibo errors
	case strings.Contains(lower, "weibo") && strings.Contains(lower, "csrf"):
		return "platform.weibo.missing_csrf"
	case strings.Contains(lower, "weibo") && strings.Contains(lower, "create failed"):
		return "platform.weibo.create_failed"
	case strings.Contains(lower, "weibo") && strings.Contains(lower, "poll retcode"):
		return "platform.weibo.poll_failed"
	case strings.Contains(lower, "weibo") && strings.Contains(lower, "login cookie"):
		return "platform.weibo.no_login_cookie"
	case strings.Contains(lower, "weibo") && strings.Contains(lower, "profile"):
		return "platform.weibo.profile_unavailable"
	case strings.Contains(lower, "weibo"):
		return "platform.weibo.error"

	// HTTP / network errors
	case strings.Contains(lower, "http 302"):
		return "platform.http_redirect"
	case strings.Contains(lower, "http 4") || strings.Contains(lower, "http 5"):
		return "platform.http_status_error"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return "platform.timeout"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host"):
		return "platform.network_error"

	default:
		return "platform.qrcode_login_failed"
	}
}
