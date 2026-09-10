package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/go-chi/chi/v5"
)

// HandleThirdPartyAccountAvatar returns one saved account avatar through a controlled platform request.
func (h *ThirdPartyHandlers) HandleThirdPartyAccountAvatar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.avatarAccounts == nil {
			httpapi.WriteError(w, r, thirdPartyCodeInternalError, nil)
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
		httpapi.WriteError(w, r, thirdPartyCodeInvalidRequest, nil)
	case errors.Is(err, thirdparty.ErrAccountNotFound):
		httpapi.WriteError(w, r, errorcodes.PlatformThirdPartyAccountNotFound, nil)
	default:
		httpapi.WriteError(w, r, thirdPartyCodeInternalError, nil)
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
		Code: errorcodes.PlatformUpstreamRequestFailed,

		Details: map[string]any{"reason": reason},
		Cause:   err,
	})
}
