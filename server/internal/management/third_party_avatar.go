package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/go-chi/chi/v5"
)

// HandleThirdPartyAccountAvatar returns one saved account avatar through a controlled platform request.
func (h *ThirdPartyHandlers) HandleThirdPartyAccountAvatar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.avatarAccounts == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, thirdPartyCodeInternalError, "三方账号头像读取不可用", "errors.platform.internal_error", nil)
			return
		}
		account, err := h.avatarAccounts.Get(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "account_id"))
		if err != nil {
			writeThirdPartyAccountAvatarLookupError(w, r, err)
			return
		}
		resource, err := thirdparty.FetchAccountAvatar(r.Context(), h.avatarClient, account.Platform, account.Profile.AvatarURL)
		if err != nil {
			writeThirdPartyAccountAvatarFetchError(w, r, account.Profile.AvatarURL, err)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", resource.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(resource.Body); err != nil {
			return
		}
	}
}

func writeThirdPartyAccountAvatarLookupError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, thirdparty.ErrInvalidAccount):
		httpapi.WriteError(w, r, http.StatusBadRequest, thirdPartyCodeInvalidRequest, "三方账号参数不正确", "errors.platform.invalid_request", nil)
	case errors.Is(err, thirdparty.ErrAccountNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "platform.third_party_account_not_found", "三方账号不存在或尚未配置凭据", "errors.platform.third_party_account_not_found", nil)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, thirdPartyCodeInternalError, "三方账号头像读取失败", "errors.platform.internal_error", nil)
	}
}

func writeThirdPartyAccountAvatarFetchError(w http.ResponseWriter, r *http.Request, avatarURL string, err error) {
	reason := "avatar_upstream_failed"
	switch {
	case strings.TrimSpace(avatarURL) == "":
		reason = "avatar_missing"
	case errors.Is(err, thirdparty.ErrAccountAvatarURLUnsupported):
		reason = "avatar_source_unsupported"
	case errors.Is(err, thirdparty.ErrAccountAvatarContentTypeUnsupported):
		reason = "avatar_content_type_unsupported"
	}
	httpapi.WriteDomainError(w, r, &httpapi.DomainError{
		Code:        "platform.upstream_request_failed",
		HTTPStatus:  http.StatusBadGateway,
		SafeMessage: "三方账号头像暂时不可用",
		MessageKey:  "errors.platform.upstream_request_failed",
		Details:     map[string]any{"reason": reason},
		Cause:       err,
	})
}
