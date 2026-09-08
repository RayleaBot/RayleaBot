package management

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *AuthHandlers) HandleAccountCredentialsUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writePermissionDenied(w, r)
			return
		}
		var request struct {
			CurrentSecret string          `json:"current_secret"`
			NewSecret     string          `json:"new_secret"`
			NewIdentifier json.RawMessage `json:"new_identifier,omitempty"`
		}
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.CurrentSecret == "" ||
			utf8.RuneCountInString(request.NewSecret) < 8 || utf8.RuneCountInString(request.NewSecret) > 1024 {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		identifier := ""
		if len(request.NewIdentifier) > 0 && (string(request.NewIdentifier) == "null" || json.Unmarshal(request.NewIdentifier, &identifier) != nil || utf8.RuneCountInString(identifier) > 128) {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		cfg := h.currentConfig()
		sourceIP := httpapi.RequestRemoteIP(r)
		if h.loginFailures != nil && !h.loginFailures.Reserve(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "platform.rate_limited", "触发平台级限流", "errors.platform.rate_limited", nil)
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
			writeAuthError(w, r, http.StatusForbidden, "permission.current_secret_invalid", "当前密码不正确", "errors.permission.current_secret_invalid")
		case errors.Is(err, auth.ErrInvalidToken), errors.Is(err, auth.ErrExpiredToken):
			writePermissionDenied(w, r)
		case errors.Is(err, auth.ErrInvalidCredentialInput):
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
		default:
			writeAuthError(w, r, http.StatusInternalServerError, authCodeInternalError, "内部错误", "errors.platform.internal_error")
		}
	}
}
