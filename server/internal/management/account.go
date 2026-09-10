package management

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *AuthHandlers) HandleAccountCredentialsUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeAuthenticationRequired(w, r)
			return
		}
		var request struct {
			CurrentSecret string          `json:"current_secret"`
			NewSecret     string          `json:"new_secret"`
			NewIdentifier json.RawMessage `json:"new_identifier,omitempty"`
		}
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.CurrentSecret == "" ||
			utf8.RuneCountInString(request.NewSecret) < 8 || utf8.RuneCountInString(request.NewSecret) > 1024 {
			writeAuthError(w, r, authCodeInvalidRequest)
			return
		}
		identifier := ""
		if len(request.NewIdentifier) > 0 && (string(request.NewIdentifier) == "null" || json.Unmarshal(request.NewIdentifier, &identifier) != nil || utf8.RuneCountInString(identifier) > 128) {
			writeAuthError(w, r, authCodeInvalidRequest)
			return
		}
		cfg := h.currentConfig()
		sourceIP := httpapi.RequestRemoteIP(r)
		if h.loginFailures != nil && !h.loginFailures.Reserve(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow) {
			httpapi.WriteError(w, r, errorcodes.PlatformRateLimited, nil)
			return
		}
		err := h.auth.UpdateCredentialsWithContext(r.Context(), claims, request.CurrentSecret, request.NewSecret, identifier)
		switch {
		case err == nil:
			if h.loginFailures != nil {
				h.loginFailures.Reset(sourceIP)
			}
			w.Header().Del(CSRFHeader)
			http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: cfg.SecureCookie,
				SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeAuthError(w, r, errorcodes.PermissionCurrentSecretInvalid)
		case errors.Is(err, auth.ErrInvalidToken), errors.Is(err, auth.ErrExpiredToken):
			writeAuthenticationRequired(w, r)
		case errors.Is(err, auth.ErrInvalidCredentialInput):
			writeAuthError(w, r, authCodeInvalidRequest)
		default:
			writeAuthError(w, r, authCodeInternalError)
		}
	}
}
